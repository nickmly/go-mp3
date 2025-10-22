build-debug:
	go build -gcflags=all="-N -l" -o debug-go-mp3.exe
run:
	go run main.go
build:
	go build -o go-mp3.exe