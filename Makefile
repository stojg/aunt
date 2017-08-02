VERSION=`git describe --tags`
BUILDTIME=`date -u +%a,\ %d\ %b\ %Y\ %H:%M:%S\ GMT`
LDFLAGS=-ldflags "-s -w -X main.Version=${VERSION} -X 'main.Compiled=${BUILDTIME}'"
BINARY=aunt

FILES = $(shell find . -type f -name '*.go' -not -path "./vendor/*")

all:
	go build ${LDFLAGS} -o ${BINARY} .

check:
	go test . ./lib/...
	goimports -d $(FILES)
	gometalinter --deadline 20s --vendor . ./lib/...

fix:
	gofmt -s -w -l $(shell find . -type f -name '*.go' -not -path "./vendor/*")
	goimports -w $(shell find . -type f -name '*.go' -not -path "./vendor/*")

install: check
	go install ${LDFLAGS} .

release: check
	GOOS=linux GOARCH=amd64 go build -o ${BINARY}_linux ${LDFLAGS} .
	#GOOS=windows GOARCH=amd64 go build -o ${BINARY}_windows ${LDFLAGS} .
	#GOOS=darwin GOARCH=amd64 go build -o ${BINARY}_darwin ${LDFLAGS} .

clean:
	if [ -f ${BINARY} ] ; then rm ${BINARY} ; fi
	if [ -f ${BINARY}_linux ] ; then rm ${BINARY}_linux ; fi
	if [ -f ${BINARY}_windows ] ; then rm ${BINARY}_windows ; fi
	if [ -f ${BINARY}_darwin ] ; then rm ${BINARY}_darwin ; fi
