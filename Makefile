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
	go install -v -x github.com/tsenart/deadcode@latest
	go install -v -x honnef.co/go/tools/cmd/staticcheck@latest
	go test -v ./...
	deadcode ./
	go vet ./...
	# Ignore the following errors:
	# - "error strings should not be capitalized"
	# - "at least one file in a package should have a package comment"
	staticcheck -checks all,-ST1005,-ST1000 ./...

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
