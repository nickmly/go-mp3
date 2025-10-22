build-debug:
	go build -gcflags=all="-N -l"
run:
	go run main.go