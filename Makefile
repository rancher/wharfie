TARGETS := $(shell ls scripts | grep -vF .sh)

.dapper:
	@echo Downloading dapper
	@DAPPER_BINARY="dapper-$$(uname -s)-$$(uname -m)"; \
	case "$$DAPPER_BINARY" in \
		dapper-Linux-x86_64)  DAPPER_SHA256="28d643818513b6bf5912922d9e35ab9a300376dce4bba8f4af59c21608b4ce65" ;; \
		dapper-Linux-aarch64) DAPPER_SHA256="31676cd981726ba24647599724261b4a70a5afb495aa4ff7d7f0d0dcd74e25e4" ;; \
		dapper-Darwin-x86_64) DAPPER_SHA256="1876b70c47c374f80c90079564cb79ada6268edc3c11a2cfca7f953bee1be4a4" ;; \
		*) echo "No pinned SHA256 for dapper on platform: $$DAPPER_BINARY" >&2; exit 1 ;; \
	esac; \
	curl -fsSL "https://releases.rancher.com/dapper/v0.5.1/$$DAPPER_BINARY" > .dapper.tmp; \
	echo "$$DAPPER_SHA256  .dapper.tmp" | sha256sum -c -
	@chmod +x .dapper.tmp
	@./.dapper.tmp -v
	@mv .dapper.tmp .dapper

$(TARGETS): .dapper
	./.dapper $@

.DEFAULT_GOAL := ci
