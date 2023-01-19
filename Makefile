.PHONY: all

-include Defaults.mk

# Linux distro (try and set to /etc/os-release ID)
OS_REL := $(shell sed -n "s/^ID\s*=\s*['"\""]\(.*\)['"\""]/\1/p" /etc/os-release)
OS ?= $(OS_REL)

# List of variables to save and replace in files
VARLIST := OS

# Project Information
VARLIST += WAREWULF VERSION RELEASE
WAREWULF ?= warewulf
VERSION ?= 4.4.0
GIT_TAG := $(shell test -e .git && git log -1 --format="%h")

ifdef GIT_TAG
  ifdef $(filter $(OS),ubuntu debian)
    RELEASE ?= 1.git_$(subst -,+,$(GIT_TAG))
  else
    RELEASE ?= 1.git_$(subst -,_,$(GIT_TAG))
  endif
else
  RELEASE ?= 1
endif

# Use LSB-compliant paths if OS is known
ifneq ($(OS),)
  USE_LSB_PATHS := true
endif

# Always default to GNU autotools default paths if PREFIX has been redefined
ifdef PREFIX
  USE_LSB_PATHS := false
endif

# System directory paths
VARLIST += PREFIX BINDIR SYSCONFDIR SRVDIR DATADIR MANDIR DOCDIR LOCALSTATEDIR
PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin
SYSCONFDIR ?= $(PREFIX)/etc
DATADIR ?= $(PREFIX)/share
MANDIR ?= $(DATADIR)/man
DOCDIR ?= $(DATADIR)/doc

ifeq ($(USE_LSB_PATHS),true)
  SRVDIR ?= /srv
  LOCALSTATEDIR ?= /var/local
else
  SRVDIR ?= $(PREFIX)/srv
  LOCALSTATEDIR ?= $(PREFIX)/var
endif

# OS-Specific Service Locations
VARLIST += TFTPDIR FIREWALLDDIR SYSTEMDDIR
SYSTEMDDIR ?= /usr/lib/systemd/system
BASHCOMPDIR ?= /etc/bash_completion.d
FIREWALLDDIR ?= /usr/lib/firewalld/services
ifeq ($(OS),suse)
  TFTPDIR ?= /srv/tftpboot
endif
ifeq ($(OS),ubuntu)
  TFTPDIR ?= /srv/tftp
endif
# Default to Red Hat / Rocky Linux
TFTPDIR ?= /var/lib/tftpboot

# Warewulf directory paths
VARLIST += WWCLIENTDIR WWCONFIGDIR WWPROVISIONDIR WWOVERLAYDIR WWCHROOTDIR WWTFTPDIR WWDOCDIR WWDATADIR TMPDIR
WWCONFIGDIR := $(SYSCONFDIR)/$(WAREWULF)
WWPROVISIONDIR := $(SRVDIR)/$(WAREWULF)
WWOVERLAYDIR := $(LOCALSTATEDIR)/$(WAREWULF)/overlays
WWCHROOTDIR := $(LOCALSTATEDIR)/$(WAREWULF)/chroots
WWTFTPDIR := $(TFTPDIR)/$(WAREWULF)
WWDOCDIR := $(DOCDIR)/$(WAREWULF)
WWDATADIR := $(DATADIR)/$(WAREWULF)
WWCLIENTDIR ?= /warewulf
TMPDIR ?= /var/tmp

# auto installed tooling
TOOLS_DIR := .tools
TOOLS_BIN := $(TOOLS_DIR)/bin
CONFIG := $(shell pwd)

# tools
GO_TOOLS_BIN := $(addprefix $(TOOLS_BIN)/, $(notdir $(GO_TOOLS)))
GO_TOOLS_VENDOR := $(addprefix vendor/, $(GO_TOOLS))
GOLANGCI_LINT := $(TOOLS_BIN)/golangci-lint
GOLANGCI_LINT_VERSION := v1.45.2

# use GOPROXY for older git clients and speed up downloads
GOPROXY ?= https://proxy.golang.org
export GOPROXY

# built tags needed for wwbuild binary
WW_GO_BUILD_TAGS := containers_image_openpgp containers_image_ostree

# Default target
all: config vendor wwctl wwclient bash_completion.d man_pages config_defaults print_defaults wwapid wwapic wwapird

# Validate source and build all packages
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

vendor:
ifndef OFFLINE_BUILD
	  go mod tidy -v
	  go mod vendor
endif

$(TOOLS_DIR):
	@mkdir -p $@

# Pre-build steps for source, such as "go generate"
config:
# Store configuration for subsequent runs
	printf " $(foreach V,$(VARLIST),$V := $(strip $($V))\n)" > Defaults.mk
    # Global variable search and replace for all *.in files
	find . -type f -name "*.in" -not -path "./vendor/*" \
		-exec sh -c 'sed -ne "$(foreach V,$(VARLIST),s,@$V@,$(strip $($V)),g;)p" $${0} > $${0%.in}' {} \;
	touch config

rm_config:
	rm -f config

genconfig: rm_config config

# Lint
lint: setup_tools
	@echo Running golangci-lint...
	@$(GOLANGCI_LINT) run --build-tags "$(WW_GO_BUILD_TAGS)" --skip-dirs internal/pkg/staticfiles ./...

vet:
	go vet ./...

test-it:
	go test -v ./... -ldflags="-X 'github.com/hpcng/warewulf/internal/pkg/warewulfconf.ConfigFile=$(shell pwd)/etc/warewulf.conf'"

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
	install -d -m 0755 $(DESTDIR)$(BINDIR)
	install -d -m 0755 $(DESTDIR)$(WWCHROOTDIR)
	install -d -m 0755 $(DESTDIR)$(WWPROVISIONDIR)
	install -d -m 0755 $(DESTDIR)$(WWOVERLAYDIR)/wwinit/$(WWCLIENTDIR)
	install -d -m 0755 $(DESTDIR)$(WWCONFIGDIR)/ipxe
	install -d -m 0755 $(DESTDIR)$(BASHCOMPDIR)
	install -d -m 0755 $(DESTDIR)$(MANDIR)/man1
	install -d -m 0755 $(DESTDIR)$(MANDIR)/man5
	install -d -m 0755 $(DESTDIR)$(WWDOCDIR)
	install -d -m 0755 $(DESTDIR)$(FIREWALLDDIR)
	install -d -m 0755 $(DESTDIR)$(SYSTEMDDIR)
	install -d -m 0755 $(DESTDIR)$(WWDATADIR)/ipxe
	test -f $(DESTDIR)$(WWCONFIGDIR)/warewulf.conf || install -m 644 etc/warewulf.conf $(DESTDIR)$(WWCONFIGDIR)
	test -f $(DESTDIR)$(WWCONFIGDIR)/nodes.conf || install -m 644 etc/nodes.conf $(DESTDIR)$(WWCONFIGDIR)
	test -f $(DESTDIR)$(WWCONFIGDIR)/wwapic.conf || install -m 644 etc/wwapic.conf $(DESTDIR)$(WWCONFIGDIR)
	test -f $(DESTDIR)$(WWCONFIGDIR)/wwapid.conf || install -m 644 etc/wwapid.conf $(DESTDIR)$(WWCONFIGDIR)
	test -f $(DESTDIR)$(WWCONFIGDIR)/wwapird.conf || install -m 644 etc/wwapird.conf $(DESTDIR)$(WWCONFIGDIR)
	test -f $(DESTDIR)$(WWCONFIGDIR)/defaults.conf || ./print_defaults > $(DESTDIR)$(WWCONFIGDIR)/defaults.conf
	cp -r etc/examples $(DESTDIR)$(WWCONFIGDIR)/
	cp -r etc/ipxe $(DESTDIR)$(WWCONFIGDIR)/
	cp -r overlays/* $(DESTDIR)$(WWOVERLAYDIR)/
	chmod 755 $(DESTDIR)$(WWOVERLAYDIR)/wwinit/init
	find $(DESTDIR)$(WWOVERLAYDIR) -type f -name "*.in" -exec rm -f {} \;
	chmod 755 $(DESTDIR)$(WWOVERLAYDIR)/wwinit/$(WWCLIENTDIR)/wwinit
	chmod 600 $(DESTDIR)$(WWOVERLAYDIR)/wwinit/etc/ssh/ssh*
	chmod 600 $(DESTDIR)$(WWOVERLAYDIR)/wwinit/etc/NetworkManager/system-connections/ww4-managed.ww
	chmod 644 $(DESTDIR)$(WWOVERLAYDIR)/wwinit/etc/ssh/ssh*.pub.ww
	chmod 750 $(DESTDIR)$(WWOVERLAYDIR)/host
	install -m 0755 wwctl $(DESTDIR)$(BINDIR)
	install -m 0755 wwapic $(DESTDIR)$(BINDIR)
	install -m 0755 wwapid $(DESTDIR)$(BINDIR)
	install -m 0755 wwapird $(DESTDIR)$(BINDIR)
	install -m 0644 include/firewalld/warewulf.xml $(DESTDIR)$(FIREWALLDDIR)
	install -m 0644 include/systemd/warewulfd.service $(DESTDIR)$(SYSTEMDDIR)
	install -m 0644 LICENSE.md $(DESTDIR)$(WWDOCDIR)
	cp bash_completion.d/warewulf $(DESTDIR)$(BASHCOMPDIR)
	cp man_pages/*.1* $(DESTDIR)$(MANDIR)/man1/
	cp man_pages/*.5* $(DESTDIR)$(MANDIR)/man5/
	install -m 0644 staticfiles/README-ipxe.md $(DESTDIR)$(WWDATADIR)/ipxe
	install -m 0644 staticfiles/arm64.efi $(DESTDIR)$(WWDATADIR)/ipxe
	install -m 0644 staticfiles/x86_64.efi $(DESTDIR)$(WWDATADIR)/ipxe
	install -m 0644 staticfiles/x86_64.kpxe $(DESTDIR)$(WWDATADIR)/ipxe

init:
	systemctl daemon-reload
	cp -r tftpboot/* $(WWTFTPDIR)/ipxe/
	restorecon -r $(WWTFTPDIR)

wwctl:
	cd cmd/wwctl; GOOS=linux go build -mod vendor -tags "$(WW_GO_BUILD_TAGS)" -o ../../wwctl

wwclient:
	cd cmd/wwclient; CGO_ENABLED=0 GOOS=linux go build -mod vendor -a -ldflags "-extldflags -static \
	 -X 'github.com/hpcng/warewulf/internal/pkg/warewulfconf.ConfigFile=/etc/warewulf/warewulf.conf'" -o ../../wwclient

install_wwclient: wwclient
	install -m 0755 wwclient $(DESTDIR)$(WWOVERLAYDIR)/wwinit/$(WWCLIENTDIR)/wwclient

bash_completion:
	cd cmd/bash_completion && go build -ldflags="-X 'github.com/hpcng/warewulf/internal/pkg/warewulfconf.ConfigFile=./etc/warewulf.conf'\
	 -X 'github.com/hpcng/warewulf/internal/pkg/node.ConfigFile=./etc/nodes.conf'"\
	 -mod vendor -tags "$(WW_GO_BUILD_TAGS)" -o ../../bash_completion

bash_completion.d: bash_completion
	install -d -m 0755 bash_completion.d
	./bash_completion bash_completion.d/warewulf

man_page:
	cd cmd/man_page && go build -ldflags="-X 'github.com/hpcng/warewulf/internal/pkg/warewulfconf.ConfigFile=./etc/warewulf.conf'\
	 -X 'github.com/hpcng/warewulf/internal/pkg/node.ConfigFile=./etc/nodes.conf'"\
	 -mod vendor -tags "$(WW_GO_BUILD_TAGS)" -o ../../man_page

man_pages: man_page
	install -d man_pages
	./man_page ./man_pages
	cp docs/man/man5/*.5 ./man_pages/
	cd man_pages; for i in wwctl*1 *.5; do echo "Compressing manpage: $$i"; gzip --force $$i; done

config_defaults: vendor cmd/config_defaults/config_defaults.go
	cd cmd/config_defaults && go build -ldflags="-X 'github.com/hpcng/warewulf/internal/pkg/warewulfconf.ConfigFile=./etc/warewulf.conf'\
	 -X 'github.com/hpcng/warewulf/internal/pkg/node.ConfigFile=./etc/nodes.conf'"\
	 -mod vendor -tags "$(WW_GO_BUILD_TAGS)" -o ../../config_defaults

print_defaults: vendor cmd/print_defaults/print_defaults.go
	cd cmd/print_defaults && go build -ldflags="-X 'github.com/hpcng/warewulf/internal/pkg/warewulfconf.ConfigFile=./etc/warewulf.conf'" -o ../../print_defaults

update_configuration: vendor cmd/update_configuration/update_configuration.go
	cd cmd/update_configuration && go build -ldflags="-X 'github.com/hpcng/warewulf/internal/pkg/warewulfconf.ConfigFile=./etc/warewulf.conf'\
	 -X 'github.com/hpcng/warewulf/internal/pkg/node.ConfigFile=./etc/nodes.conf'"\
	 -mod vendor -tags "$(WW_GO_BUILD_TAGS)" -o ../../update_configuration

warewulfconf: config_defaults
	./config_defaults

dist: vendor config
	rm -rf .dist/$(WAREWULF)-$(VERSION)
	mkdir -p .dist/$(WAREWULF)-$(VERSION)
	cp -rap * .dist/$(WAREWULF)-$(VERSION)/
	find .dist/$(WAREWULF)-$(VERSION)/ -type f -name '*~' -delete
	cd .dist; tar -czf ../$(WAREWULF)-$(VERSION).tar.gz $(WAREWULF)-$(VERSION)
	rm -rf .dist

## wwapi generate code from protobuf. Requires protoc and protoc-grpc-gen-gateway to generate code.
## To setup latest protoc:
##    Download the protobuf-all-[VERSION].tar.gz from https://github.com/protocolbuffers/protobuf/releases
##    Extract the contents and change in the directory
##    ./configure
##    make
##    make check
##    sudo make install
##    sudo ldconfig # refresh shared library cache.
## To setup protoc-gen-grpc-gateway, see https://github.com/grpc-ecosystem/grpc-gateway
proto:
	rm -rf internal/pkg/api/routes/wwapiv1/
	protoc -I internal/pkg/api/routes/v1 -I=. \
		--grpc-gateway_out=. \
		--grpc-gateway_opt logtostderr=true \
		--go_out=. \
		--go-grpc_out=. \
		routes.proto

wwapid: ## Build the grpc api server.
	go build -o ./wwapid internal/app/api/wwapid/wwapid.go

wwapic: ## Build the sample wwapi client.
	go build -o ./wwapic  internal/app/api/wwapic/wwapic.go

wwapird: ## Build the rest api server (revese proxy to the grpc api server).
	go build -o ./wwapird internal/app/api/wwapird/wwapird.go

clean:
	rm -f wwclient
	rm -f wwctl
	rm -rf .dist
	rm -f $(WAREWULF)-$(VERSION).tar.gz
	rm -f bash_completion
	rm -rf bash_completion.d
	rm -f man_page
	rm -rf man_pages
	rm -rf vendor
	rm -f warewulf.spec
	rm -f config
	rm -f Defaults.mk
	rm -rf $(TOOLS_DIR)
	rm -f config_defaults
	rm -f update_configuration
	rm -f print_defaults
	rm -f etc/wwapi{c,d,rd}.conf

install: files install_wwclient

debinstall: files debfiles
