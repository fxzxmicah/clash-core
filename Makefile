NAME=clash-core
BUILDDIR=build/releases
VERSION=$(shell git describe --tags || echo v0.0.0-test)
BUILDTIME=$(shell date -u)
GOBUILD=CGO_ENABLED=0 go build -trimpath -ldflags '-X "github.com/Dreamacro/clash/constant.Version=$(VERSION)" \
		-X "github.com/Dreamacro/clash/constant.BuildTime=$(BUILDTIME)" \
		-w -s -buildid='

LINUX_ARCH_LIST = \
	linux-386 \
	linux-amd64 \
	linux-armv7 \
	linux-arm64 \
	linux-mips-softfloat \
	linux-mips-hardfloat \
	linux-mipsle-softfloat \
	linux-mipsle-hardfloat \
	linux-mips64 \
	linux-mips64le \
	linux-riscv64

WINDOWS_ARCH_LIST = \
	windows-386 \
	windows-amd64 \
	windows-arm64 \
	windows-armv7

all: linux-amd64 windows-amd64 # Most used

linux-386:
	GOARCH=386 GOOS=linux $(GOBUILD) -o $(BUILDDIR)/../$(NAME)-$@

linux-amd64:
	GOARCH=amd64 GOOS=linux $(GOBUILD) -o $(BUILDDIR)/../$(NAME)-$@

linux-armv7:
	GOARCH=arm GOOS=linux GOARM=7 $(GOBUILD) -o $(BUILDDIR)/../$(NAME)-$@

linux-arm64:
	GOARCH=arm64 GOOS=linux $(GOBUILD) -o $(BUILDDIR)/../$(NAME)-$@

linux-mips-softfloat:
	GOARCH=mips GOMIPS=softfloat GOOS=linux $(GOBUILD) -o $(BUILDDIR)/../$(NAME)-$@

linux-mips-hardfloat:
	GOARCH=mips GOMIPS=hardfloat GOOS=linux $(GOBUILD) -o $(BUILDDIR)/../$(NAME)-$@

linux-mipsle-softfloat:
	GOARCH=mipsle GOMIPS=softfloat GOOS=linux $(GOBUILD) -o $(BUILDDIR)/../$(NAME)-$@

linux-mipsle-hardfloat:
	GOARCH=mipsle GOMIPS=hardfloat GOOS=linux $(GOBUILD) -o $(BUILDDIR)/../$(NAME)-$@

linux-mips64:
	GOARCH=mips64 GOOS=linux $(GOBUILD) -o $(BUILDDIR)/../$(NAME)-$@

linux-mips64le:
	GOARCH=mips64le GOOS=linux $(GOBUILD) -o $(BUILDDIR)/../$(NAME)-$@

linux-riscv64:
	GOARCH=riscv64 GOOS=linux $(GOBUILD) -o $(BUILDDIR)/../$(NAME)-$@

windows-386:
	GOARCH=386 GOOS=windows $(GOBUILD) -o $(BUILDDIR)/../$(NAME)-$@.exe

windows-amd64:
	GOARCH=amd64 GOOS=windows GOAMD64=v3 $(GOBUILD) -o $(BUILDDIR)/../$(NAME)-$@.exe

windows-arm64:
	GOARCH=arm64 GOOS=windows $(GOBUILD) -o $(BUILDDIR)/../$(NAME)-$@.exe

windows-armv7:
	GOARCH=arm GOOS=windows GOARM=7 $(GOBUILD) -o $(BUILDDIR)/../$(NAME)-$@.exe

linux_tar_releases=$(addsuffix .tar.gz, $(LINUX_ARCH_LIST))

$(linux_tar_releases): %.tar.gz : %
	tar -c -z -f $(BUILDDIR)/$(NAME)-$(patsubst %.tar.gz,%,$@)-$(VERSION).tar.gz -C $(BUILDDIR)/../ $(NAME)-$(patsubst %.tar.gz,%,$@)

windows_cab_releases=$(addsuffix .cab, $(WINDOWS_ARCH_LIST))

$(windows_cab_releases): %.cab : %
	makecab $(BUILDDIR)/../$(NAME)-$(basename $@).exe $(BUILDDIR)/$(NAME)-$(basename $@)-$(VERSION).cab

all-arch: $(LINUX_ARCH_LIST) $(WINDOWS_ARCH_LIST)

releases: $(linux_tar_releases) $(windows_cab_releases)

LINT_OS_LIST := windows linux

lint: $(foreach os,$(LINT_OS_LIST),$(os)-lint)
%-lint:
	GOOS=$* golangci-lint run ./...

lint-fix: $(foreach os,$(LINT_OS_LIST),$(os)-lint-fix)
%-lint-fix:
	GOOS=$* golangci-lint run --fix ./...

clean:
	rm $(BUILDDIR)/*
	rm $(BUILDDIR)/../*
