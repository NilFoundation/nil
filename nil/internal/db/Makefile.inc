.PHONY: ssz_db
ssz_db: nil/internal/db/tables_encoding.go

nil/internal/db/tables_encoding.go: nil/internal/db/tables.go nil/common/hash.go nil/common/length.go nil/internal/types/message.go
	cd nil/internal/db && go generate