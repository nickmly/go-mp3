build-debug:
	go build -gcflags=all="-N -l"
run:
	go run main.go
build:
	go build -o go-mp3.exe