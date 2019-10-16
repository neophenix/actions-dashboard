package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var user string
var pass string
var port string
var org string
var topic string
var include string
var exclude string
var divWidth string

var httpClient http.Client

type repoInfo struct {
	Name     string   `json:"name"`
	FullName string   `json:"full_name"`
	HTMLUrl  string   `json:"html_url"`
	Topics   []string `json:"topics"`
}

type commitInfo struct {
	Sha string `json:"sha"`
}

type checkRunResponse struct {
	TotalCount int            `json:"total_count"`
	CheckRuns  []checkRunInfo `json:"check_runs"`
}
type checkRunInfo struct {
	Name        string `json:"name"`
	Status      string `json:"status"`
	Conclusion  string `json:"conclusion"`
	CompletedAt string `json:"completed_at"`
}

type repoStatus struct {
	Name   string
	Status string
	URL    string
	Time   string
}

func main() {
	flag.StringVar(&user, "user", "", "github username")
	flag.StringVar(&pass, "pass", "", "github password or access token")
	flag.StringVar(&port, "port", "8080", "port to listen on")
	flag.StringVar(&org, "org", "", "organization to pull repos from, blank for your own")
	flag.StringVar(&topic, "topics", "", "topics (csv) to include from repo list, this is an or, so any match will include the repo")
	flag.StringVar(&include, "include", "", "repositories (csv) to look at for check-run status")
	flag.StringVar(&exclude, "exclude", "", "repositories (csv) to exclude looking at (useful if you are getting repos by topic)")
	flag.StringVar(&divWidth, "width", "200px", "the repo status div min-width style")
	flag.Parse()

	httpClient = http.Client{Timeout: 10 * time.Second}

	c := make(chan repoStatus)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		repos := getRepos()
		for _, repo := range repos {
			go doRepoWork(repo, c)
		}

		s := []repoStatus{}
		for i := 0; i < len(repos); i++ {
			resp := <-c
			s = append(s, resp)
		}
		fmt.Fprintf(w, "<html><head>")
		fmt.Fprintf(w, "<style>")
		fmt.Fprintf(w, "body { background-color: black; color: white; }")
		fmt.Fprintf(w, "a { color: white; text-decoration: none; }")
		fmt.Fprintf(w, "div { margin: 5px; padding: 15px; float: left; min-width: %v }", divWidth)
		fmt.Fprintf(w, ".success { background-color: #259225; }")
		fmt.Fprintf(w, ".unknown { background-color: #9d9d9d; }")
		fmt.Fprintf(w, ".failure { background-color: #eb0000; }")
		fmt.Fprintf(w, "</style>")
		fmt.Fprintf(w, "</head><body>")
		for _, v := range s {
			fmt.Fprintf(w, `<a href="%s" target="_blank"><div class="%s"><p>%s</p><p>%s</p><p>%s</p></div></a>`, v.URL, v.Status, v.Name, timeSince(v.Time), v.Time)
		}
		fmt.Fprintf(w, "</body></html>")
	})

	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func getRepos() []repoInfo {
	var body []byte
	if org != "" {
		body = httpRequest("https://api.github.com/orgs/" + org + "/repos")
	} else if user != "" {
		body = httpRequest("https://api.github.com/users/" + user + "/repos")
	} else {
		log.Fatal("Either user or org is required")
	}

	// change our CSVs into maps for easier lookup
	includes := make(map[string]bool)
	excludes := make(map[string]bool)
	topics := make(map[string]bool)
	for _, v := range strings.Split(include, ",") {
		includes[v] = true
	}
	for _, v := range strings.Split(exclude, ",") {
		excludes[v] = true
	}
	for _, v := range strings.Split(topic, ",") {
		topics[v] = true
	}

	allRepos := []repoInfo{}
	err := json.Unmarshal(body, &allRepos)
	if err != nil {
		log.Fatalf("Error parsing repo list: %v", err)
	}

	// loop over the repos and only return the ones we care about
	list := []repoInfo{}
	for _, repo := range allRepos {
		// skip excludes
		if !excludes[repo.Name] {
			// include if its on the include list
			if includes[repo.Name] {
				list = append(list, repo)
			} else {
				// include if it has a matching topic
				for _, t := range repo.Topics {
					if topics[t] {
						list = append(list, repo)
					}
				}
			}
		}
	}

	return list
}

func doRepoWork(repo repoInfo, c chan<- repoStatus) {
	sha := getShaOfLastCommit(repo.FullName)
	runInfo := getCheckRunStatusOfLastCommit(repo.FullName, sha)
	status := repoStatus{
		Name:   repo.Name,
		Status: "unknown",
		URL:    repo.HTMLUrl,
		Time:   "",
	}
	if runInfo != nil {
		status.Status = runInfo.Conclusion
		status.Time = runInfo.CompletedAt
	}

	c <- status
}

func getShaOfLastCommit(repo string) string {
	body := httpRequest("https://api.github.com/repos/" + repo + "/commits")

	commits := []commitInfo{}
	err := json.Unmarshal(body, &commits)
	if err != nil {
		log.Fatalf("Error parsing commit list: %v", err)
	}

	return commits[0].Sha
}

func getCheckRunStatusOfLastCommit(repo string, sha string) *checkRunInfo {
	body := httpRequest("https://api.github.com/repos/" + repo + "/commits/" + sha + "/check-runs")

	info := checkRunResponse{}
	err := json.Unmarshal(body, &info)
	if err != nil {
		log.Fatalf("Error parsing check-run info: %v", err)
	}

	if info.TotalCount > 0 {
		return &info.CheckRuns[0]
	}
	return nil
}

func httpRequest(url string) []byte {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatalf("%v", err)
	}

	if user != "" && pass != "" {
		req.SetBasicAuth(user, pass)
	}

	// needed for repo topics, check-run status
	req.Header.Add("Accept", "application/vnd.github.mercy-preview+json,application/vnd.github.antiope-preview+json")

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Fatalf("Error performing get %v: %v", url, err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Error reading %v response body: %v", url, err)
	}

	return body
}

func timeSince(t string) string {
	if t == "" {
		return ""
	}

	count := 0
	unit := ""

	ts, _ := time.Parse(time.RFC3339, t)
	since := time.Since(ts)
	if since.Seconds() >= 86400*30 {
		count = int(since.Seconds() / (86400 * 30))
		unit = "month"
	} else if since.Seconds() >= 86400 {
		count = int(since.Seconds() / 86400)
		unit = "day"
	} else if since.Seconds() >= 3600 {
		count = int(since.Seconds() / 3600)
		unit = "hour"
	} else if since.Seconds() >= 60 {
		count = int(since.Seconds() / 60)
		unit = "minute"
	} else {
		count = int(since.Seconds())
		unit = "second"
	}

	if count > 1 {
		unit += "s"
	}
	return strconv.Itoa(count) + " " + unit + " ago"
}
