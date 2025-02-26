.DEFAULT_GOAL := help

.PHONY: all
all: build

##@ General

# https://gist.github.com/prwhite/8168133
# Maybe use https://github.com/drdv/makefile-doc in the future
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "The Warewulf Makefile\n\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
	@echo
	@echo "Define OFFLINE_BUILD=1 to avoid requiring network access."

include Variables.mk
include Tools.mk

##@ Build

.PHONY: build
build: wwctl wwclient etc/bash_completion.d/wwctl ## Build the Warewulf binaries

.PHONY: api
api: wwapid wwapic wwapird

.PHONY: docs
docs: man_pages reference ## Build the documentation

.PHONY: spec
spec: warewulf.spec ## Create an RPM spec file

.PHONY: dist
dist: $(config) ## Create a distributable source tarball
	rm -rf .dist/
	mkdir -p .dist/$(WAREWULF)-$(VERSION)
	git ls-files >.dist/dist-files
	tar -c --files-from .dist/dist-files | tar -C .dist/$(WAREWULF)-$(VERSION) -x
	test -d vendor/ && cp -a vendor/ .dist/$(WAREWULF)-$(VERSION) || :
	scripts/get-version.sh >.dist/$(WAREWULF)-$(VERSION)/VERSION
	tar -C .dist -czf $(WAREWULF)-$(VERSION).tar.gz $(WAREWULF)-$(VERSION)
	rm -rf .dist/

config = include/systemd/warewulfd.service \
	internal/pkg/config/buildconfig.go \
	warewulf.spec
.PHONY: config
config: $(config)

apiconfig = etc/wwapic.conf \
	etc/wwapid.conf \
	etc/wwapird.conf
.PHONY: apiconfig
apiconfig: $(apiconfig)

%: %.in
	sed -ne "$(foreach V,$(VARLIST),s,@$V@,$(strip $($V)),g;)p" $@.in >$@

wwctl: $(config) $(call godeps,cmd/wwctl/main.go)
	GOOS=linux go build -mod vendor -tags "$(WW_GO_BUILD_TAGS)" -o wwctl cmd/wwctl/main.go

wwclient: $(config) $(call godeps,cmd/wwclient/main.go)
	CGO_ENABLED=0 GOOS=linux go build -mod vendor -a -ldflags "-extldflags -static" -o wwclient cmd/wwclient/main.go

wwapid: $(config) $(apiconfig) $(call godeps,internal/app/api/wwapid/wwapid.go)
	go build -o ./wwapid internal/app/api/wwapid/wwapid.go

wwapic: $(config) $(apiconfig) $(call godeps,internal/app/api/wwapic/wwapic.go)
	go build -o ./wwapic  internal/app/api/wwapic/wwapic.go

wwapird: $(config) $(apiconfig) $(call godeps,internal/app/api/wwapird/wwapird.go)
	go build -o ./wwapird internal/app/api/wwapird/wwapird.go

.PHONY: man_pages
man_pages: wwctl $(wildcard docs/man/man5/*.5)
	mkdir -p docs/man/man1
	WWWORKER=8 ./wwctl --emptyconf genconfig man docs/man/man1
	gzip --force docs/man/man1/*.1
	for manpage in docs/man/man5/*.5; do gzip <$${manpage} >$${manpage}.gz; done

etc/bash_completion.d/wwctl: wwctl
	mkdir -p etc/bash_completion.d/
	./wwctl --emptyconf completion bash >etc/bash_completion.d/wwctl

.PHONY: reference
reference: wwctl
	mkdir -p userdocs/reference
	./wwctl --emptyconf genconfig reference userdocs/reference/

latexpdf: reference
	make -C userdocs latexpdf

##@ Development

vendor: ## Create the vendor directory (if it does not exist)
	go mod vendor

.PHONY: tidy
tidy: ## Clean up golang dependencies
	go mod tidy

.PHONY: fmt
fmt: ## Update source code formatting
	go fmt ./...

.PHONY: lint
lint: $(config) ## Run the linter
	$(GOLANGCI_LINT) run --build-tags "$(WW_GO_BUILD_TAGS)" --timeout=5m ./...

.PHONY: staticcheck
staticcheck: $(GOLANG_STATICCHECK) $(config) ## Run static code check
	$(GOLANG_STATICCHECK) ./...

.PHONY: deadcode
deadcode: $(config) ## Check for unused code
	$(GOLANG_DEADCODE) -test ./...

.PHONY: vet
vet: $(config) ## Check for invalid code
	go vet ./...

.PHONY: test
test: $(config) ## Run full test suite
	TZ=UTC go test ./...

.PHONY: test-cover
test-cover: $(config) ## Generate a coverage report for the test suite
	rm -rf coverage
	mkdir coverage
	go list -f '{{if gt (len .TestGoFiles) 0}}"TZ=UTC go test -covermode count -coverprofile {{.Name}}.coverprofile -coverpkg ./... {{.ImportPath}}"{{end}}' ./... | xargs -I {} bash -c {}
	echo "mode: count" >coverage/cover.out
	grep -h -v "^mode:" *.coverprofile >>"coverage/cover.out"
	rm *.coverprofile
	go tool cover -html=coverage/cover.out -o=coverage/cover.html

.PHONY: LICENSE_DEPENDENCIES.md
LICENSE_DEPENDENCIES.md: $(GOLANG_LICENSES) scripts/update-license-dependencies.sh
	rm -rf vendor
	GOLANG_LICENSES=$(GOLANG_LICENSES) scripts/update-license-dependencies.sh

.PHONY: licenses
licenses: LICENSE_DEPENDENCIES.md # Update LICENSE_DEPENDENCIES.md

.PHONY: cleanconfig
cleanconfig:
	rm -f $(config)
	rm -rf etc/bash_completion.d/

.PHONY: cleantest
cleantest:
	rm -rf *.coverprofile

.PHONY: cleandist
cleandist:
	rm -f $(WAREWULF)-$(VERSION).tar.gz
	rm -rf .dist/

.PHONY: cleanmake
cleanmake:
	rm -f Defaults.mk

.PHONY: cleanbin
cleanbin:
	rm -f wwapi{c,d,rd}
	rm -f wwclient
	rm -f wwctl
	rm -f update_configuration

.PHONY: cleandocs
cleandocs:
	rm -rf userdocs/_*
	rm -rf userdocs/reference/*
	rm -rf docs/man/man1
	rm -rf docs/man/man5/*.gz

.PHONY: cleanvendor
cleanvendor:
	rm -rf vendor

.PHONY: clean
clean: cleanconfig cleantest cleandist cleantools cleanmake cleanbin cleandocs ## Remove built configuration, docs, binaries, and artifacts

##@ Installation

.PHONY: install
install: build docs ## Install Warewulf from source
	install -d -m 0755 $(DESTDIR)$(BINDIR)
	install -d -m 0755 $(DESTDIR)$(WWCHROOTDIR)
	install -d -m 0755 $(DESTDIR)$(WWOVERLAYDIR)
	install -d -m 0755 $(DESTDIR)$(WWPROVISIONDIR)
	install -d -m 0755 $(DESTDIR)$(DATADIR)/warewulf/overlays/wwinit/rootfs/$(WWCLIENTDIR)
	install -d -m 0755 $(DESTDIR)$(DATADIR)/warewulf/overlays/wwclient/rootfs/$(WWCLIENTDIR)
	install -d -m 0755 $(DESTDIR)$(DATADIR)/warewulf/overlays/host/rootfs/$(TFTPDIR)/warewulf/
	install -d -m 0755 $(DESTDIR)$(WWCONFIGDIR)/examples
	install -d -m 0755 $(DESTDIR)$(WWCONFIGDIR)/ipxe
	install -d -m 0755 $(DESTDIR)$(WWCONFIGDIR)/grub
	install -d -m 0755 $(DESTDIR)$(BASHCOMPDIR)
	install -d -m 0755 $(DESTDIR)$(MANDIR)/man1
	install -d -m 0755 $(DESTDIR)$(MANDIR)/man5
	install -d -m 0755 $(DESTDIR)$(WWDOCDIR)
	install -d -m 0755 $(DESTDIR)$(FIREWALLDDIR)
	install -d -m 0755 $(DESTDIR)$(LOGROTATEDIR)
	install -d -m 0755 $(DESTDIR)$(SYSTEMDDIR)
	install -d -m 0755 $(DESTDIR)$(IPXESOURCE)
	install -d -m 0755 $(DESTDIR)$(DATADIR)/warewulf
	# wwctl genconfig to get the compiled in paths to warewulf.conf
	install -d -m 0755 $(DESTDIR)$(DATADIR)/warewulf/bmc
	test -f $(DESTDIR)$(WWCONFIGDIR)/warewulf.conf || ./wwctl --warewulfconf etc/warewulf.conf genconfig warewulfconf print> $(DESTDIR)$(WWCONFIGDIR)/warewulf.conf
	test -f $(DESTDIR)$(WWCONFIGDIR)/nodes.conf || install -m 0644 etc/nodes.conf $(DESTDIR)$(WWCONFIGDIR)
	for f in etc/examples/*.ww; do install -m 0644 $$f $(DESTDIR)$(WWCONFIGDIR)/examples/; done
	for f in etc/ipxe/*.ipxe; do install -m 0644 $$f $(DESTDIR)$(WWCONFIGDIR)/ipxe/; done
	for f in lib/warewulf/bmc/*.tmpl; do install -m 0644 $$f $(DESTDIR)$(DATADIR)/warewulf/bmc; done
	install -m 0644 etc/grub/grub.cfg.ww $(DESTDIR)$(WWCONFIGDIR)/grub/grub.cfg.ww
	install -m 0644 etc/grub/chainload.ww $(DESTDIR)$(DATADIR)/warewulf/overlays/host/rootfs$(TFTPDIR)/warewulf/grub.cfg.ww
	install -m 0644 etc/logrotate.d/warewulfd.conf $(DESTDIR)$(LOGROTATEDIR)/warewulfd.conf
	(cd overlays && find * -path '*/internal' -prune -o -type f -exec install -D -m 0644 {} $(DESTDIR)$(DATADIR)/warewulf/overlays/{} \;)
	(cd overlays && find * -path '*/internal' -prune -o -type d -exec mkdir -pv $(DESTDIR)$(DATADIR)/warewulf/overlays/{} \;)
	(cd overlays && find * -path '*/internal' -prune -o -type l -exec cp -av {} $(DESTDIR)$(DATADIR)/warewulf/overlays/{} \;)
	chmod 0755 $(DESTDIR)$(DATADIR)/warewulf/overlays/wwinit/rootfs/init
	chmod 0755 $(DESTDIR)$(DATADIR)/warewulf/overlays/wwinit/rootfs/$(WWCLIENTDIR)/wwprescripts
	chmod 0600 $(DESTDIR)$(DATADIR)/warewulf/overlays/wwinit/rootfs/$(WWCLIENTDIR)/config.ww
	chmod 0600 $(DESTDIR)$(DATADIR)/warewulf/overlays/ssh.host_keys/rootfs/etc/ssh/ssh*
	chmod 0644 $(DESTDIR)$(DATADIR)/warewulf/overlays/ssh.host_keys/rootfs/etc/ssh/ssh*.pub.ww
	chmod 0600 $(DESTDIR)$(DATADIR)/warewulf/overlays/NetworkManager/rootfs/etc/NetworkManager/system-connections/ww4-managed.ww
	chmod 0750 $(DESTDIR)$(DATADIR)/warewulf/overlays/host/rootfs
	install -m 0755 wwctl $(DESTDIR)$(BINDIR)
	install -m 0755 wwclient $(DESTDIR)$(DATADIR)/warewulf/overlays/wwinit/rootfs/$(WWCLIENTDIR)/wwclient
	install -m 0644 include/firewalld/warewulf.xml $(DESTDIR)$(FIREWALLDDIR)
	install -m 0644 include/systemd/warewulfd.service $(DESTDIR)$(SYSTEMDDIR)
	install -m 0644 LICENSE.md $(DESTDIR)$(WWDOCDIR)
	install -m 0644 etc/bash_completion.d/wwctl $(DESTDIR)$(BASHCOMPDIR)/wwctl
	for f in docs/man/man1/*.1.gz; do install -m 0644 $$f $(DESTDIR)$(MANDIR)/man1/; done
	for f in docs/man/man5/*.5.gz; do install -m 0644 $$f $(DESTDIR)$(MANDIR)/man5/; done
	install -pd -m 0755 $(DESTDIR)$(DRACUTMODDIR)/90wwinit
	install -m 0644 dracut/modules.d/90wwinit/*.sh $(DESTDIR)$(DRACUTMODDIR)/90wwinit

.PHONY: installapi
installapi:
	install -m 0755 wwapic $(DESTDIR)$(BINDIR)
	install -m 0755 wwapid $(DESTDIR)$(BINDIR)
	install -m 0755 wwapird $(DESTDIR)$(BINDIR)
	test -f $(DESTDIR)$(WWCONFIGDIR)/wwapic.conf || install -m 0644 etc/wwapic.conf $(DESTDIR)$(WWCONFIGDIR)
	test -f $(DESTDIR)$(WWCONFIGDIR)/wwapid.conf || install -m 0644 etc/wwapid.conf $(DESTDIR)$(WWCONFIGDIR)
	test -f $(DESTDIR)$(WWCONFIGDIR)/wwapird.conf || install -m 0644 etc/wwapird.conf $(DESTDIR)$(WWCONFIGDIR)

.PHONY: init
init:
	systemctl daemon-reload
	cp -r tftpboot/* $(WWTFTPDIR)/ipxe/
	restorecon -r $(WWTFTPDIR)

ifndef OFFLINE_BUILD
wwctl: vendor
wwclient: vendor
update_configuration: vendor
wwapid: vendor
wwapic: vendor
wwapird: vendor
dist: vendor

lint: $(GOLANGCI_LINT)
deadcode: $(GOLANG_DEADCODE)

protofiles = internal/pkg/api/routes/wwapiv1/routes.pb.go \
	internal/pkg/api/routes/wwapiv1/routes.pb.gw.go \
	internal/pkg/api/routes/wwapiv1/routes_grpc.pb.go
.PHONY: proto
proto: $(protofiles)

routes_proto = internal/pkg/api/routes/v1/routes.proto
$(protofiles): $(routes_proto) $(PROTOC) $(PROTOC_GEN_GRPC_GATEWAY) $(PROTOC_GEN_GO) $(PROTOC_GEN_GO_GRPC)
	PATH=$(TOOLS_BIN):$(PATH) $(PROTOC) \
		-I /usr/include -I $(shell dirname $(routes_proto)) -I=. \
		--grpc-gateway_opt logtostderr=true \
		--go_out=. \
		--go-grpc_out=. \
		--grpc-gateway_out=. \
		routes.proto

.PHONY: cleanproto
cleanproto:
	rm -f $(protofiles)

clean: cleanvendor
endif
