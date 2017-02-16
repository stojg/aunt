
build:
	go fmt .
	go vet .
	go test .
	go install .
	aunt

js:
	./node_modules/.bin/webpack -d -w --progress

compile:
	./node_modules/.bin/webpack -p --progress
	go install .
