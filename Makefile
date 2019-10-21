VERSION=$(shell grep "var Version" shared/version/version.go | cut -d'"' -f2)
ARCHIVE=distrobuilder-$(VERSION).tar

.PHONY: default check dist

default:
	gofmt -s -w .
	go get -t -v -d ./...
	go install -v ./...
	@echo "distrobuilder built successfully"

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
	cd $(TMP)/distrobuilder-$(VERSION) && GOPATH=$(TMP)/_dist go get -t -v -d ./...

	# Write a manifest
	cd $(TMP)/_dist && find . -type d -name .git | while read line; do GITDIR=$$(dirname $$line); echo "$${GITDIR}: $$(cd $${GITDIR} && git show-ref HEAD $${GITDIR} | cut -d' ' -f1)"; done | sort > $(TMP)/_dist/MANIFEST

	# Assemble tarball
	rm $(TMP)/_dist/src/github.com/lxc/distrobuilder
	ln -s ../../../../ $(TMP)/_dist/src/github.com/lxc/distrobuilder
	mv $(TMP)/_dist $(TMP)/distrobuilder-$(VERSION)/
	tar --exclude-vcs -C $(TMP) -zcf $(ARCHIVE).gz distrobuilder-$(VERSION)/

	# Cleanup
	rm -Rf $(TMP)
