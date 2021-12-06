.PHONY: all

VERSION ?= 4.2.0
RELEASE ?= 1

SRC ?= main

# auto installed tooling
TOOLS_DIR := .tools
TOOLS_BIN := $(TOOLS_DIR)/bin

# tools
GO_TOOLS_BIN := $(addprefix $(TOOLS_BIN)/, $(notdir $(GO_TOOLS)))
GO_TOOLS_VENDOR := $(addprefix vendor/, $(GO_TOOLS))
GOLANGCI_LINT := $(TOOLS_BIN)/golangci-lint
GOLANGCI_LINT_VERSION := v1.31.0

# use GOPROXY for older git clients and speed up downloads
GOPROXY ?= https://proxy.golang.org
export GOPROXY

# built tags needed for wwbuild binary
WW_BUILD_GO_BUILD_TAGS := containers_image_openpgp containers_image_ostree

# system-overlay directories for install_wwclient
OVERLAY_DIR ?= $(DESTDIR)/var/warewulf/overlays/system/
overlays = ${dir ${wildcard ${OVERLAY_DIR}*/}}

all: vendor wwctl wwclient bash_completion man_page

build: lint test-it vet all

# set the go tools into the tools bin.
setup_tools: $(GO_TOOLS_BIN) $(GOLANGCI_LINT)

# install go tools into TOOLS_BIN
$(GO_TOOLS_BIN):
	@GOBIN="$(PWD)/$(TOOLS_BIN)" go install -mod=vendor $(GO_TOOLS)

# install golangci-lint into TOOLS_BIN
$(GOLANGCI_LINT):
	@curl -qq -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(TOOLS_BIN) $(GOLANGCI_LINT_VERSION)


setup: vendor $(TOOLS_DIR) setup_tools

# vendor
vendor:
	go mod tidy -v
	go mod vendor

$(TOOLS_DIR):
	@mkdir -p $@

# Lint
lint: setup_tools
	@echo Running golangci-lint...
	@$(GOLANGCI_LINT) run --build-tags "$(WW_BUILD_GO_BUILD_TAGS)" --skip-dirs internal/pkg/staticfiles ./...

vet:
	go vet ./...

test-it:
	go test -v ./...

# Generate test coverage
test-cover:     ## Run test coverage and generate html report
	rm -fr coverage
	mkdir coverage
	go list -f '{{if gt (len .TestGoFiles) 0}}"go test -covermode count -coverprofile {{.Name}}.coverprofile -coverpkg ./... {{.ImportPath}}"{{end}}' ./... | xargs -I {} bash -c {}
	echo "mode: count" > coverage/cover.out
	grep -h -v "^mode:" *.coverprofile >> "coverage/cover.out"
	rm *.coverprofile
	go tool cover -html=coverage/cover.out -o=coverage/cover.html

debian: all 

files: all
	install -d -m 0755 $(DESTDIR)/usr/bin/
	install -d -m 0755 $(DESTDIR)/var/warewulf/
	install -d -m 0755 $(DESTDIR)/var/warewulf/chroots
	install -d -m 0755 $(DESTDIR)/etc/warewulf/
	install -d -m 0755 $(DESTDIR)/etc/warewulf/ipxe
	install -d -m 0755 $(DESTDIR)/var/lib/tftpboot/warewulf/ipxe/
	install -d -m 0755 $(DESTDIR)/etc/bash_completion.d/
	install -d -m 0755 $(DESTDIR)/usr/share/man/man1
	test -f $(DESTDIR)/etc/warewulf/warewulf.conf || install -m 644 etc/warewulf.conf $(DESTDIR)/etc/warewulf/
	test -f $(DESTDIR)/etc/warewulf/hosts.tmpl || install -m 644 etc/hosts.tmpl $(DESTDIR)/etc/warewulf/
	test -f $(DESTDIR)/etc/warewulf/nodes.conf || install -m 640 etc/nodes.conf $(DESTDIR)/etc/warewulf/
	cp -r etc/dhcp $(DESTDIR)/etc/warewulf/
	cp -r etc/ipxe $(DESTDIR)/etc/warewulf/
	cp -r overlays $(DESTDIR)/var/warewulf/
	chmod +x $(DESTDIR)/var/warewulf/overlays/system/default/init
	chmod 600 $(DESTDIR)/var/warewulf/overlays/system/default/etc/ssh/ssh*
	chmod 644 $(DESTDIR)/var/warewulf/overlays/system/default/etc/ssh/ssh*.pub.ww
	mkdir -p $(DESTDIR)/var/warewulf/overlays/system/default/warewulf/bin/
	cp wwctl $(DESTDIR)/usr/bin/
	mkdir -p $(DESTDIR)/usr/lib/firewalld/services
	install -c -m 0644 include/firewalld/warewulf.xml $(DESTDIR)/usr/lib/firewalld/services
	mkdir -p $(DESTDIR)/usr/lib/systemd/system
	install -c -m 0644 include/systemd/warewulfd.service $(DESTDIR)/usr/lib/systemd/system
	cp etc/bash_completion.d/warewulf $(DESTDIR)/etc/bash_completion.d/
	cp usr/share/man/man1/* $(DESTDIR)/usr/share/man/man1/

init:
	systemctl daemon-reload
	cp -r tftpboot/* /var/lib/tftpboot/warewulf/ipxe/
	restorecon -r /var/lib/tftpboot/warewulf

debfiles: debian
	chmod +x $(DESTDIR)/var/warewulf/overlays/system/debian/init
	chmod 600 $(DESTDIR)/var/warewulf/overlays/system/debian/etc/ssh/ssh*
	chmod 644 $(DESTDIR)/var/warewulf/overlays/system/debian/etc/ssh/ssh*.pub.ww
	mkdir -p $(DESTDIR)/var/warewulf/overlays/system/debian/warewulf/bin/
	cp wwclient $(DESTDIR)/var/warewulf/overlays/system/debian/warewulf/bin/

wwctl:
	cd cmd/wwctl; GOOS=linux go build -mod vendor -tags "$(WW_BUILD_GO_BUILD_TAGS)" -o ../../wwctl

wwclient:
	cd cmd/wwclient; CGO_ENABLED=0 GOOS=linux go build -mod vendor -a -ldflags '-extldflags -static' -o ../../wwclient

install_wwclient: wwclient
	$(foreach overlay, ${overlays}, install -D -m 0755 wwclient ${overlay}warewulf/bin/wwclient;)

bash_completion:
	cd cmd/bash_completion && go build -mod vendor -tags "$(WW_BUILD_GO_BUILD_TAGS)" -o ../../bash_completion
	install -d -m 0755 ./etc/bash_completion.d/
	./bash_completion  ./etc/bash_completion.d/warewulf

man_page:
	cd cmd/man_page && go build -mod vendor -tags "$(WW_BUILD_GO_BUILD_TAGS)" -o ../../man_page
	install -d -m 0755 ./usr/share/man/man1
	./man_page ./usr/share/man/man1
	gzip --force ./usr/share/man/man1/wwctl*1

dist: vendor
	rm -rf _dist/warewulf-$(VERSION)
	mkdir -p _dist/warewulf-$(VERSION)
	git archive --format=tar $(SRC) | tar -xf - -C _dist/warewulf-$(VERSION)
	cp -r vendor _dist/warewulf-$(VERSION)/
	sed -e 's/@VERSION@/$(VERSION)/g; s/@RELEASE@/$(RELEASE)/g' warewulf.spec.in > _dist/warewulf-$(VERSION)/warewulf.spec
	cd _dist; tar -czf ../warewulf-$(VERSION).tar.gz warewulf-$(VERSION)

clean:
	rm -f wwclient
	rm -f wwctl
	rm -rf _dist
	rm -f warewulf-$(VERSION).tar.gz
	rm -f bash_completion
	rm -f man_page

install: files install_wwclient

debinstall: files debfiles

