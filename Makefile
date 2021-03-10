SHELL=/usr/bin/env bash

PG_IMAGE?=postgres:10
REDIS_IMAGE?=redis:6
COMMIT := $(shell git rev-parse --short HEAD)

# GITVERSION is the nearest tag plus number of commits and short form of most recent commit since the tag, if any
GITVERSION=$(shell git describe --always --tag --dirty)

unexport GOFLAGS

CLEAN:=
BINS:=

GOFLAGS:=

## FFI

FFI_PATH:=extern/filecoin-ffi/
FFI_DEPS:=.install-filcrypto
FFI_DEPS:=$(addprefix $(FFI_PATH),$(FFI_DEPS))

$(FFI_DEPS): build/.filecoin-install ;

build/.filecoin-install: $(FFI_PATH)
	$(MAKE) -C $(FFI_PATH) $(FFI_DEPS:$(FFI_PATH)%=%)
	@touch $@

MODULES+=$(FFI_PATH)
BUILD_DEPS+=build/.filecoin-install
CLEAN+=build/.filecoin-install

ffi-version-check:
	@[[ "$$(awk '/const Version/{print $$5}' extern/filecoin-ffi/version.go)" -eq 2 ]] || (echo "FFI version mismatch, update submodules"; exit 1)
BUILD_DEPS+=ffi-version-check

.PHONY: ffi-version-check


$(MODULES): build/.update-modules ;
# dummy file that marks the last time modules were updated
build/.update-modules:
	git submodule update --init --recursive
	touch $@

CLEAN+=build/.update-modules
# end git modules


ldflags=-X=github.com/filecoin-project/sentinel-visor/version.GitVersion=$(GITVERSION)
ifneq ($(strip $(LDFLAGS)),)
	ldflags+=-extldflags=$(LDFLAGS)
endif
GOFLAGS+=-ldflags="$(ldflags)"

.PHONY: all
all: build

.PHONY: build
build: deps visor


.PHONY: deps
deps: $(BUILD_DEPS)
	cd ./vector; ./fetch_vectors.sh

# test starts dependencies and runs all tests
.PHONY: test
test: testfull

.PHONY: dockerup
dockerup:
	docker-compose up -d

.PHONY: dockerdown
dockerdown:
	docker-compose down

# testfull runs all tests
.PHONY: testfull
testfull: build
	docker-compose up -d
	sleep 2
	./visor migrate --latest
	TZ= PGSSLMODE=disable go test ./... -v || echo ""
	docker-compose down

# testshort runs tests that don't require external dependencies such as postgres or redis
.PHONY: testshort
testshort:
	go test -short ./... -v

# lint runs linting against code base
.PHONY: lint
lint:
	go run github.com/golangci/golangci-lint/cmd/golangci-lint run

.PHONY: visor
visor:
	rm -f visor
	go build $(GOFLAGS) -o visor -mod=readonly .

BINS+=visor

.PHONY: docker-image
docker-image:
	docker build -t "filecoin/sentinel-visor" .
	docker tag "filecoin/sentinel-visor:latest" "filecoin/sentinel-visor:$(COMMIT)"

clean:
	rm -rf $(CLEAN) $(BINS)
.PHONY: clean

vector-clean:
	rm ./vector/data/*json

dist-clean:
	git clean -xdff
	git submodule deinit --all -f
.PHONY: dist-clean

test-coverage:
	VISOR_TEST_DB="postgres://postgres:password@localhost:5432/postgres?sslmode=disable" go test -coverprofile=coverage.out ./...
.PHONY: test-coverage

