# aunt

Aunt is a application that can proxy typical monitoring stats that are hard to get or find via (graphite). It aims
 to be deployed as a web server and can run on the command line.
 
# Installation

Via the go lang get command

`go get github.com/stojg/aunt`

Downloading a binary from the https://github.com/stojg/aunt/releases

# Usage

Start aunt as a web server running on port 8080

`aunt --port 8080`

and go to the index web page at http://localhost:8080/
 
You can also run it as a CLI tool with `aunt`.

# Notes

It takes a while for aunt to query AWS cloudformation data, it typically takes around
30 seconds before it have cached the first run in memory.

# Todo

* Add multi AWS account support by assuming roles 
* Clean up the duplicate code by the use of interfaces
* Add filtering and sorting
* Add compile timestamp, version and run time to the index point
* Show self monitoring stats, such as last time fetched etc
* Setup subcommands for self installation
* Store configuration data in JSON
* Add makefile build target for doing a release to the main repo






 
