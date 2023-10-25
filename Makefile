init:
	go mod download
	go get ./...

# this is only needed until I modularise this codebase
run:
	go run pkg/gui/ui.go -p default
