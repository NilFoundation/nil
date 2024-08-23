.PHONY: ssz_config
ssz_config: nil/internal/config/params_encoding.go

nil/internal/config/params_encoding.go: nil/internal/config/params.go nil/internal/config/generate.go
	cd nil/internal/config && go generate
