# actions-dashboard

I needed a quick view of the latest "status" of my CI via GitHub Actions, sort of like the dashboard you get in Jenkins, etc.  So this is the outcome of that, takes some command line params and then hits the GitHub API for the latest commit + status when you load the page

## Usage
```
    -port     (default:8080) which port to listen on
    -user     your GitHub username
    -pass     your GitHub password or access token (used in basic auth)
    -org      the organization to list repos from
    -topics   a list (csv) of topics to match from for repos to include in output, this is an or so any match wins
    -include  a list (csv) of repos to include for checking
    -exclude  a list (csv) or repos to exclude
```

Ex.
```
./actions-dashboard -user neophenix -pass MYTOKEN -topics foo -exclude actions-dashboard
```
