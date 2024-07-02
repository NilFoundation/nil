.PHONY: ssz_db
ssz_db: core/db/tables_encoding.go

core/db/tables_encoding.go: core/db/tables.go common/hash.go common/length.go core/types/message.go
	cd core/db && go generate