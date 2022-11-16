all:
	go build -o mini
	rm -f ~/go/bin/mini
	cp -p mini ~/go/bin/
