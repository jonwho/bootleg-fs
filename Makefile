GOCMD=go

all: build
	./bootleg-fs
build:
	$(GOCMD) build
