.PHONY: ssz_types
ssz_types: core/types/account_encoding.go core/types/block_encoding.go core/types/collator_encoding.go core/types/log_encoding.go core/types/message_encoding.go core/types/receipt_encoding.go core/types/version_info_encoding.go core/types/messagestatus_string.go

core/types/log_encoding.go: core/types/log.go common/length.go core/types/address.go common/hash.go core/types/block.go
	cd core/types && go generate

core/types/receipt_encoding.go: core/types/receipt.go common/length.go core/types/address.go core/types/block.go core/types/bloom.go core/types/log.go common/hash.go
	cd core/types && go generate

core/types/message_encoding.go: core/types/message.go common/length.go core/types/address.go core/types/uint256.go core/types/code.go core/types/shard.go core/types/bloom.go core/types/log.go common/hash.go core/types/signature.go core/types/account.go
	cd core/types && go generate

core/types/block_encoding.go: core/types/block.go common/length.go core/types/address.go core/types/uint256.go core/types/code.go core/types/shard.go core/types/bloom.go core/types/log.go core/types/message.go common/hash.go
	cd core/types && go generate

core/types/collator_encoding.go: core/types/collator.go core/types/shard.go core/types/block.go core/types/message.go
	cd core/types && go generate

core/types/account_encoding.go: core/types/account.go common/length.go core/types/message.go core/types/address.go core/types/uint256.go core/types/code.go core/types/shard.go core/types/bloom.go core/types/log.go common/hash.go
	cd core/types && go generate

core/types/version_info_encoding.go: core/types/version_info.go common/hash.go common/length.go
	cd core/types && go generate

core/types/messagestatus_string.go: core/types/message.go
	cd core/types && go generate
