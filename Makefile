VERSION=$(shell grep "var Version" shared/version/version.go | cut -d'"' -f2)
ARCHIVE=distrobuilder-$(VERSION).tar
GO111MODULE=on

.PHONY: default check dist

default:
	gofmt -s -w .
	go install -v ./...
	@echo "distrobuilder built successfully"

update-gomod:
	go get -t -v -d -u ./...
	go mod tidy

check: default
	go get -v -x github.com/remyoudompheng/go-misc/deadcode
	go get -v -x golang.org/x/lint/golint
	go test -v ./...
	golint -set_exit_status ./...
	deadcode ./
	go vet ./...

dist:
	# Cleanup
	rm -Rf $(ARCHIVE).gz

	# Create build dir
	$(eval TMP := $(shell mktemp -d))
	git archive --prefix=distrobuilder-$(VERSION)/ HEAD | tar -x -C $(TMP)
	mkdir -p $(TMP)/_dist/src/github.com/lxc
	ln -s ../../../../distrobuilder-$(VERSION) $(TMP)/_dist/src/github.com/lxc/distrobuilder

	# Download dependencies
	cd $(TMP)/distrobuilder-$(VERSION) && go mod vendor

	# Assemble tarball
	tar --exclude-vcs -C $(TMP) -zcf $(ARCHIVE).gz distrobuilder-$(VERSION)/

	# Cleanup
	rm -Rf $(TMP)
