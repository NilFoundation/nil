.PHONY: ssz_types
ssz_types: nil/internal/types/account_encoding.go nil/internal/types/block_encoding.go nil/internal/types/collator_encoding.go nil/internal/types/log_encoding.go nil/internal/types/transaction_encoding.go nil/internal/types/receipt_encoding.go nil/internal/types/version_info_encoding.go nil/internal/types/error_string.go

nil/internal/types/log_encoding.go: nil/internal/types/log.go nil/common/length.go nil/internal/types/address.go nil/common/hash.go nil/internal/types/block.go
	cd nil/internal/types && go generate log.go

nil/internal/types/receipt_encoding.go: nil/internal/types/receipt.go nil/common/length.go nil/internal/types/address.go nil/internal/types/block.go nil/internal/types/bloom.go nil/internal/types/log.go nil/common/hash.go
	cd nil/internal/types && go generate receipt.go

nil/internal/types/transaction_encoding.go: nil/internal/types/transaction.go nil/common/length.go nil/internal/types/address.go nil/internal/types/uint256.go nil/internal/types/code.go nil/internal/types/shard.go nil/internal/types/bloom.go nil/internal/types/log.go nil/common/hash.go nil/internal/types/signature.go nil/internal/types/account.go
	cd nil/internal/types && go generate transaction.go

nil/internal/types/block_encoding.go: nil/internal/types/block.go nil/common/length.go nil/internal/types/address.go nil/internal/types/uint256.go nil/internal/types/code.go nil/internal/types/shard.go nil/internal/types/bloom.go nil/internal/types/log.go nil/internal/types/transaction.go nil/common/hash.go
	cd nil/internal/types && go generate block.go

nil/internal/types/collator_encoding.go: nil/internal/types/collator.go nil/internal/types/shard.go nil/internal/types/block.go nil/internal/types/transaction.go
	cd nil/internal/types && go generate collator.go

nil/internal/types/account_encoding.go: nil/internal/types/account.go nil/common/length.go nil/internal/types/transaction.go nil/internal/types/address.go nil/internal/types/uint256.go nil/internal/types/code.go nil/internal/types/shard.go nil/internal/types/bloom.go nil/internal/types/log.go nil/common/hash.go
	cd nil/internal/types && go generate account.go

nil/internal/types/version_info_encoding.go: nil/internal/types/version_info.go nil/common/hash.go nil/common/length.go
	cd nil/internal/types && go generate version_info.go

nil/internal/types/error_string.go: nil/internal/types/exec_errors.go
	cd nil/internal/types && go generate exec_errors.go
