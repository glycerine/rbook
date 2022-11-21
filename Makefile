all: githash
	go generate
	go clean
	go build -o rbook
	rm -f ~/go/bin/rbook
	cp -p rbook ~/go/bin/


githash:
	/bin/echo "package main" > gitcommit.go
	/bin/echo "func init() { LAST_GIT_COMMIT_HASH = \"$(shell git rev-parse HEAD)\"; NEAREST_GIT_TAG= \"$(shell git describe --abbrev=0 --tags)\"; GIT_BRANCH=\"$(shell git rev-parse --abbrev-ref  HEAD)\"; GO_VERSION=\"$(shell go version)\";}" >> gitcommit.go
