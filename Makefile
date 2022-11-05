clean:
	rm -f instance-stack-curator

build:
	go build -ldflags='-s -w' -o instance-stack-curator main.go

.PHONY: clean build
