.PHONY: ssz_execution
ssz_execution: nil/internal/execution/proposal_encoding.go ssz_types

nil/internal/execution/proposal_encoding.go: nil/internal/execution/proposal.go
	cd nil/internal/execution && go run github.com/NilFoundation/fastssz/sszgen --path proposal.go \
		-include ../../common/hexutil/bytes.go,../types/uint256.go,../types/bitflags.go,../types/bloom.go,../types/gas.go,../types/value.go,../types/address.go,../types/code.go,../types/account.go,../types/signature.go,../types/block.go,../types/collator.go,../types/transaction.go,../types/shard.go,../../common/length.go,../../common/hash.go,../../common/sszx/map.go \
		--objs ParentBlockSerializable,ProposalSerializable
