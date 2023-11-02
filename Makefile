GOFLAGS := -v

export GOPATH := $(CURDIR)/.go


default: all
.PHONY: default


.PHONY: all
all: diablo


diablo: FORCE
	go build $(GOFLAGS) -o $@


.PHONY: clean
clean:
	-rm diablo


.PHONY: tidy
tidy: clean
	-chmod -R 700 $(GOPATH)
	-rm -rf $(GOPATH)


FORCE: ;
