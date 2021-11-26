#!make

upx := $(shell which upx)
outName = "tiny-http-proxy"

run:
	go run .

build:
	go build -ldflags "-w -s" -o ${outName} .
ifdef upx
	upx ${outName}
endif
