package mpttracer

import (
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
	pb "github.com/NilFoundation/nil/nil/services/synccommittee/prover/proto"
	"github.com/NilFoundation/nil/nil/services/synccommittee/prover/tracer/internal/constants"
)

func TracesFromProto(pbMptTraces *pb.MPTTraces) (*MPTTraces, error) {
	check.PanicIfNot(pbMptTraces != nil)

	addrToStorageTraces := make(map[types.Address][]StorageTrieUpdateTrace,
		len(pbMptTraces.GetStorageTracesByAccount()))
	for addr, pbStorageTrace := range pbMptTraces.GetStorageTracesByAccount() {
		storageTraces := make([]StorageTrieUpdateTrace, len(pbStorageTrace.GetUpdatesTraces()))
		for i, pbStorageUpdateTrace := range pbStorageTrace.GetUpdatesTraces() {
			proof, err := protoToGoProof(pbStorageUpdateTrace.GetSerializedProof())
			if err != nil {
				return nil, err
			}
			storageTraces[i] = StorageTrieUpdateTrace{
				Key:         common.HexToHash(pbStorageUpdateTrace.GetKey()),
				RootBefore:  common.HexToHash(pbStorageUpdateTrace.GetRootBefore()),
				RootAfter:   common.HexToHash(pbStorageUpdateTrace.GetRootAfter()),
				ValueBefore: pb.ProtoUint256ToUint256(pbStorageUpdateTrace.GetValueBefore()),
				ValueAfter:  pb.ProtoUint256ToUint256(pbStorageUpdateTrace.GetValueAfter()),
				Proof:       proof,
				// PathBefore: // Unused
				// PathAfter:  // Unused
			}
		}
		addrToStorageTraces[types.HexToAddress(addr)] = storageTraces
	}

	contractTrieTraces := make([]ContractTrieUpdateTrace, len(pbMptTraces.GetContractTrieTraces()))
	for i, pbContractTrieUpdate := range pbMptTraces.GetContractTrieTraces() {
		proof, err := protoToGoProof(pbContractTrieUpdate.GetSerializedProof())
		if err != nil {
			return nil, err
		}
		contractTrieTraces[i] = ContractTrieUpdateTrace{
			Key:         common.HexToHash(pbContractTrieUpdate.GetKey()),
			RootBefore:  common.HexToHash(pbContractTrieUpdate.GetRootBefore()),
			RootAfter:   common.HexToHash(pbContractTrieUpdate.GetRootAfter()),
			ValueBefore: smartContractFromProto(pbContractTrieUpdate.GetValueBefore()),
			ValueAfter:  smartContractFromProto(pbContractTrieUpdate.GetValueAfter()),
			Proof:       proof,
			// PathBefore: // Unused
			// PathAfter:  // Unused
		}
	}

	ret := &MPTTraces{
		StorageTracesByAccount: addrToStorageTraces,
		ContractTrieTraces:     contractTrieTraces,
	}

	return ret, nil
}

func TracesToProto(mptTraces *MPTTraces, traceIdx uint64) (*pb.MPTTraces, error) {
	check.PanicIfNot(mptTraces != nil)

	pbAddrToStorageTraces := make(map[string]*pb.StorageTrieUpdatesTraces, len(mptTraces.StorageTracesByAccount))
	for addr, storageTraces := range mptTraces.StorageTracesByAccount {
		pbStorageTraces := make([]*pb.StorageTrieUpdateTrace, len(storageTraces))
		for i, storageUpdateTrace := range storageTraces {
			proof, err := goToProtoProof(storageUpdateTrace.Proof)
			if err != nil {
				return nil, err
			}

			pbStorageTraces[i] = &pb.StorageTrieUpdateTrace{
				Key:             storageUpdateTrace.Key.Hex(),
				RootBefore:      storageUpdateTrace.RootBefore.Hex(),
				RootAfter:       storageUpdateTrace.RootAfter.Hex(),
				ValueBefore:     pb.Uint256ToProtoUint256(storageUpdateTrace.ValueBefore),
				ValueAfter:      pb.Uint256ToProtoUint256(storageUpdateTrace.ValueAfter),
				SerializedProof: proof,
				ProofBefore:     storageUpdateTrace.PathBefore,
				ProofAfter:      storageUpdateTrace.PathAfter,
			}
		}

		pbAddrToStorageTraces[addr.Hex()] = &pb.StorageTrieUpdatesTraces{UpdatesTraces: pbStorageTraces}
	}

	pbContractTrieTraces := make([]*pb.ContractTrieUpdateTrace, len(mptTraces.ContractTrieTraces))
	for i, contractTrieUpdate := range mptTraces.ContractTrieTraces {
		proof, err := goToProtoProof(contractTrieUpdate.Proof)
		if err != nil {
			return nil, err
		}

		pbContractTrieTraces[i] = &pb.ContractTrieUpdateTrace{
			Key:             contractTrieUpdate.Key.Hex(),
			RootBefore:      contractTrieUpdate.RootBefore.Hex(),
			RootAfter:       contractTrieUpdate.RootAfter.Hex(),
			ValueBefore:     smartContractToProto(contractTrieUpdate.ValueBefore),
			ValueAfter:      smartContractToProto(contractTrieUpdate.ValueAfter),
			SerializedProof: proof,
			ProofBefore:     contractTrieUpdate.PathBefore,
			ProofAfter:      contractTrieUpdate.PathAfter,
		}
	}

	ret := &pb.MPTTraces{
		StorageTracesByAccount: pbAddrToStorageTraces,
		ContractTrieTraces:     pbContractTrieTraces,
		TraceIdx:               traceIdx,
		ProtoHash:              constants.ProtoHash,
	}

	return ret, nil
}

func goToProtoProof(p mpt.Proof) ([]byte, error) {
	encodedProof, err := p.Encode()
	if err != nil {
		return nil, err
	}
	return encodedProof, nil
}

func protoToGoProof(pbProof []byte) (mpt.Proof, error) {
	return mpt.DecodeProof(pbProof)
}

// smartContractFromProto converts a Protocol Buffers SmartContract to Go SmartContract
func smartContractFromProto(pbSmartContract *pb.SmartContract) *types.SmartContract {
	if pbSmartContract == nil {
		return nil
	}

	var balance types.Value
	if pbSmartContract.GetBalance() != nil {
		b := pb.ProtoUint256ToUint256(pbSmartContract.GetBalance())
		balance = types.NewValue(b.Int())
	}
	return &types.SmartContract{
		Address:          types.HexToAddress(pbSmartContract.GetAddress()),
		Balance:          types.NewValue(balance.Int()),
		TokenRoot:        common.HexToHash(pbSmartContract.GetTokenRoot()),
		StorageRoot:      common.HexToHash(pbSmartContract.GetStorageRoot()),
		CodeHash:         common.HexToHash(pbSmartContract.GetCodeHash()),
		AsyncContextRoot: common.HexToHash(pbSmartContract.GetAsyncContextRoot()),
		Seqno:            types.Seqno(pbSmartContract.GetSeqno()),
		ExtSeqno:         types.Seqno(pbSmartContract.GetExtSeqno()),
	}
}

// smartContractToProto converts a Go SmartContract to Protocol Buffers SmartContract
func smartContractToProto(smartContract *types.SmartContract) *pb.SmartContract {
	if smartContract == nil {
		return nil
	}

	var pbBalance *pb.Uint256
	if smartContract.Balance.Uint256 != nil {
		pbBalance = pb.Uint256ToProtoUint256(smartContract.Balance.Uint256)
	}
	return &pb.SmartContract{
		Address:          smartContract.Address.Hex(),
		Balance:          pbBalance,
		TokenRoot:        smartContract.TokenRoot.Hex(),
		StorageRoot:      smartContract.StorageRoot.Hex(),
		CodeHash:         smartContract.CodeHash.Hex(),
		AsyncContextRoot: smartContract.AsyncContextRoot.Hex(),
		Seqno:            uint64(smartContract.Seqno),
		ExtSeqno:         uint64(smartContract.ExtSeqno),
	}
}
