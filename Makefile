all:
	go generate
	go clean
	go build -o rbook
	rm -f ~/go/bin/rbook
	cp -p rbook ~/go/bin/
