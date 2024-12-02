package tracer

import (
	"os"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/internal/vm"
	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/mpttracer"
	pb "github.com/NilFoundation/nil/nil/services/synccommittee/prover/proto"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/proto"
)

// Set of pb messages splitted by circuits
type PbTracesSet struct {
	bytecode *pb.BytecodeTraces
	rw       *pb.RWTraces
	zkevm    *pb.ZKEVMTraces
	copy     *pb.CopyTraces
	mpt      *pb.MPTTraces
}

// Each message is serialized into file with corresponding extension added to base file path
const (
	bytecodeExtension = ".bc"
	rwExtension       = ".rw"
	zkevmExtension    = ".zkevm"
	copyExtension     = ".copy"
	mptExtension      = ".mpt"
)

func marshalToFile[Msg proto.Message](msg Msg, filename string) error {
	data, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0o600)
}

func unmarshalFromFile[Msg proto.Message](filename string, out Msg) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	return proto.Unmarshal(data, out)
}

func SerializeToFile(proofs ExecutionTraces, baseFileName string) error {
	// Convert ExecutionTraces to protobuf messages set
	pbTraces, err := ToProto(proofs)
	if err != nil {
		return err
	}

	// Write trace files in parallel
	eg := errgroup.Group{} // TODO: use WithContext to cancel remaining jobs in case of error

	// Marshal zkevm message
	eg.Go(func() error {
		return marshalToFile(pbTraces.zkevm, baseFileName+zkevmExtension)
	})

	// Marshal bytecode message
	eg.Go(func() error {
		return marshalToFile(pbTraces.bytecode, baseFileName+bytecodeExtension)
	})

	// Marshal rw message
	eg.Go(func() error {
		return marshalToFile(pbTraces.rw, baseFileName+rwExtension)
	})

	// Marshal copy message
	eg.Go(func() error {
		return marshalToFile(pbTraces.copy, baseFileName+copyExtension)
	})

	// Marshal msg traces message
	eg.Go(func() error {
		return marshalToFile(pbTraces.mpt, baseFileName+mptExtension)
	})

	return eg.Wait()
}

func DeserializeFromFile(baseFileName string) (ExecutionTraces, error) {
	pbTraces := PbTracesSet{
		bytecode: &pb.BytecodeTraces{},
		rw:       &pb.RWTraces{},
		zkevm:    &pb.ZKEVMTraces{},
		copy:     &pb.CopyTraces{},
		mpt:      &pb.MPTTraces{},
	}

	// Unmarshal trace files in parallel
	eg := errgroup.Group{}

	// Unmarshal zkevm message
	eg.Go(func() error {
		return unmarshalFromFile(baseFileName+zkevmExtension, pbTraces.zkevm)
	})

	// Unmarshal bc message
	eg.Go(func() error {
		return unmarshalFromFile(baseFileName+bytecodeExtension, pbTraces.bytecode)
	})

	// Unmarshal rw message
	eg.Go(func() error {
		return unmarshalFromFile(baseFileName+rwExtension, pbTraces.rw)
	})

	// Unmarshal copy message
	eg.Go(func() error {
		return unmarshalFromFile(baseFileName+copyExtension, pbTraces.copy)
	})

	// Unmarshal mpt traces message
	eg.Go(func() error {
		return unmarshalFromFile(baseFileName+mptExtension, pbTraces.mpt)
	})

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	// Convert protobuf messages back to ExecutionTraces
	return FromProto(&pbTraces)
}

func FromProto(traces *PbTracesSet) (ExecutionTraces, error) {
	ep := &executionTracesImpl{
		StackOps:          make([]StackOp, len(traces.rw.StackOps)),
		MemoryOps:         make([]MemoryOp, len(traces.rw.MemoryOps)),
		StorageOps:        make([]StorageOp, len(traces.rw.StorageOps)),
		ZKEVMStates:       make([]ZKEVMState, len(traces.zkevm.ZkevmStates)),
		ContractsBytecode: make(map[types.Address][]byte, len(traces.bytecode.ContractBytecodes)),
		CopyEvents:        make([]CopyEvent, len(traces.copy.CopyEvents)),
	}

	for i, pbStackOp := range traces.rw.StackOps {
		ep.StackOps[i] = StackOp{
			IsRead: pbStackOp.IsRead,
			Idx:    int(pbStackOp.Index),
			Value:  protoUint256ToUint256(pbStackOp.Value),
			PC:     pbStackOp.Pc,
			MsgId:  uint(pbStackOp.MsgId),
			RwIdx:  uint(pbStackOp.RwIdx),
		}
	}

	for i, pbMemOp := range traces.rw.MemoryOps {
		ep.MemoryOps[i] = MemoryOp{
			IsRead: pbMemOp.IsRead,
			Idx:    int(pbMemOp.Index),
			Value:  pbMemOp.Value[0],
			PC:     pbMemOp.Pc,
			MsgId:  uint(pbMemOp.MsgId),
			RwIdx:  uint(pbMemOp.RwIdx),
		}
	}

	for i, pbStorageOp := range traces.rw.StorageOps {
		ep.StorageOps[i] = StorageOp{
			IsRead:    pbStorageOp.IsRead,
			Key:       common.HexToHash(pbStorageOp.Key),
			Value:     protoUint256ToUint256(pbStorageOp.Value),
			PrevValue: protoUint256ToUint256(pbStorageOp.PrevValue),
			PC:        pbStorageOp.Pc,
			MsgId:     uint(pbStorageOp.MsgId),
			RwIdx:     uint(pbStorageOp.RwIdx),
			Addr:      types.HexToAddress(pbStorageOp.Address.String()),
		}
	}

	for i, pbZKEVMState := range traces.zkevm.ZkevmStates {
		ep.ZKEVMStates[i] = ZKEVMState{
			TxHash:          common.HexToHash(pbZKEVMState.TxHash),
			TxId:            int(pbZKEVMState.CallId),
			PC:              pbZKEVMState.Pc,
			Gas:             pbZKEVMState.Gas,
			RwIdx:           uint(pbZKEVMState.RwIdx),
			BytecodeHash:    common.HexToHash(pbZKEVMState.BytecodeHash),
			OpCode:          vm.OpCode(pbZKEVMState.Opcode),
			AdditionalInput: protoUint256ToUint256(pbZKEVMState.AdditionalInput),
			StackSize:       pbZKEVMState.StackSize,
			MemorySize:      pbZKEVMState.MemorySize,
			TxFinish:        pbZKEVMState.TxFinish,
			StackSlice:      make([]types.Uint256, len(pbZKEVMState.StackSlice)),
			MemorySlice:     make(map[uint64]uint8),
			StorageSlice:    make(map[types.Uint256]types.Uint256),
		}

		for j, stackVal := range pbZKEVMState.StackSlice {
			ep.ZKEVMStates[i].StackSlice[j] = protoUint256ToUint256(stackVal)
		}
		for addr, memVal := range pbZKEVMState.MemorySlice {
			ep.ZKEVMStates[i].MemorySlice[addr] = uint8(memVal)
		}
		for _, entry := range pbZKEVMState.StorageSlice {
			key := protoUint256ToUint256(entry.Key)
			ep.ZKEVMStates[i].StorageSlice[key] = protoUint256ToUint256(entry.Value)
		}
	}

	for i, pbCopyEventTrace := range traces.copy.GetCopyEvents() {
		ep.CopyEvents[i].From = copyParticipantFromProto(pbCopyEventTrace.From)
		ep.CopyEvents[i].To = copyParticipantFromProto(pbCopyEventTrace.To)
		ep.CopyEvents[i].RwIdx = uint(pbCopyEventTrace.RwIdx)
		ep.CopyEvents[i].Data = pbCopyEventTrace.GetData()
	}

	for pbContractAddr, pbContractBytecode := range traces.bytecode.ContractBytecodes {
		ep.ContractsBytecode[types.HexToAddress(pbContractAddr)] = pbContractBytecode
	}

	mptTraces, err := mptTracesFromProto(traces.mpt)
	if err != nil {
		return nil, err
	}
	ep.MPTTraces = mptTraces

	return ep, nil
}

func ToProto(tr ExecutionTraces) (*PbTracesSet, error) {
	traces, ok := tr.(*executionTracesImpl)
	if !ok {
		panic("Unexpected traces type")
	}
	pbTraces := &PbTracesSet{
		bytecode: &pb.BytecodeTraces{ContractBytecodes: make(map[string][]byte)},
		rw: &pb.RWTraces{
			StackOps:   make([]*pb.StackOp, len(traces.StackOps)),
			MemoryOps:  make([]*pb.MemoryOp, len(traces.MemoryOps)),
			StorageOps: make([]*pb.StorageOp, len(traces.StorageOps)),
		},
		zkevm: &pb.ZKEVMTraces{ZkevmStates: make([]*pb.ZKEVMState, len(traces.ZKEVMStates))},
		copy:  &pb.CopyTraces{CopyEvents: make([]*pb.CopyEvent, len(traces.CopyEvents))},
	}

	// Convert StackOps
	for i, stackOp := range traces.StackOps {
		pbTraces.rw.StackOps[i] = &pb.StackOp{
			IsRead: stackOp.IsRead,
			Index:  int32(stackOp.Idx),
			Value:  uint256ToProtoUint256(stackOp.Value),
			Pc:     stackOp.PC,
			MsgId:  uint64(stackOp.MsgId),
			RwIdx:  uint64(stackOp.RwIdx),
		}
	}

	// Convert MemoryOps
	for i, memOp := range traces.MemoryOps {
		pbTraces.rw.MemoryOps[i] = &pb.MemoryOp{
			IsRead: memOp.IsRead,
			Index:  int32(memOp.Idx),
			Value:  []byte{memOp.Value},
			Pc:     memOp.PC,
			MsgId:  uint64(memOp.MsgId),
			RwIdx:  uint64(memOp.RwIdx),
		}
	}

	// Convert StorageOps
	for i, storageOp := range traces.StorageOps {
		pbTraces.rw.StorageOps[i] = &pb.StorageOp{
			IsRead:    storageOp.IsRead,
			Key:       storageOp.Key.Hex(),
			Value:     uint256ToProtoUint256(storageOp.Value),
			PrevValue: uint256ToProtoUint256(storageOp.PrevValue),
			Pc:        storageOp.PC,
			MsgId:     uint64(storageOp.MsgId),
			RwIdx:     uint64(storageOp.RwIdx),
			Address:   &pb.Address{AddressBytes: storageOp.Addr.Bytes()},
		}
	}

	for i, zkevmState := range traces.ZKEVMStates {
		pbTraces.zkevm.ZkevmStates[i] = &pb.ZKEVMState{
			TxHash:          zkevmState.TxHash.Hex(),
			CallId:          uint64(zkevmState.TxId),
			Pc:              zkevmState.PC,
			Gas:             zkevmState.Gas,
			RwIdx:           uint64(zkevmState.RwIdx),
			BytecodeHash:    zkevmState.BytecodeHash.String(),
			Opcode:          uint64(zkevmState.OpCode),
			AdditionalInput: uint256ToProtoUint256(zkevmState.AdditionalInput),
			StackSize:       zkevmState.StackSize,
			MemorySize:      zkevmState.MemorySize,
			TxFinish:        zkevmState.TxFinish,
			StackSlice:      make([]*pb.Uint256, len(zkevmState.StackSlice)),
			MemorySlice:     make(map[uint64]uint32),
			StorageSlice:    make([]*pb.StorageEntry, len(zkevmState.StorageSlice)),
		}
		for j, stackVal := range zkevmState.StackSlice {
			pbTraces.zkevm.ZkevmStates[i].StackSlice[j] = uint256ToProtoUint256(stackVal)
		}
		for addr, memVal := range zkevmState.MemorySlice {
			pbTraces.zkevm.ZkevmStates[i].MemorySlice[addr] = uint32(memVal)
		}
		storageSliceCounter := 0
		for storageKey, storageVal := range zkevmState.StorageSlice {
			pbEntry := &pb.StorageEntry{
				Key:   uint256ToProtoUint256(storageKey),
				Value: uint256ToProtoUint256(storageVal),
			}
			pbTraces.zkevm.ZkevmStates[i].StorageSlice[storageSliceCounter] = pbEntry
			storageSliceCounter++
		}
	}

	for i, copyEvent := range traces.CopyEvents {
		pbTraces.copy.CopyEvents[i] = &pb.CopyEvent{
			From:  copyParticipantToProto(&copyEvent.From),
			To:    copyParticipantToProto(&copyEvent.To),
			RwIdx: uint64(copyEvent.RwIdx),
			Data:  copyEvent.Data,
		}
	}

	// Convert ContractsBytecode
	for addr, bytecode := range traces.ContractsBytecode {
		pbTraces.bytecode.ContractBytecodes[addr.Hex()] = bytecode
	}

	mptTraces, err := mptTracesToProto(traces.MPTTraces)
	if err != nil {
		return nil, err
	}
	pbTraces.mpt = mptTraces

	return pbTraces, nil
}

func uint256ToProtoUint256(u types.Uint256) *pb.Uint256 {
	return &pb.Uint256{
		WordParts: u[:],
	}
}

func protoUint256ToUint256(pb *pb.Uint256) types.Uint256 {
	var u types.Uint256
	copy(u[:], pb.WordParts)
	return u
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

var copyLocationToProtoMap = map[CopyLocation]pb.CopyLocation{
	CopyLocationMemory:     pb.CopyLocation_MEMORY,
	CopyLocationBytecode:   pb.CopyLocation_BYTECODE,
	CopyLocationCalldata:   pb.CopyLocation_CALLDATA,
	CopyLocationLog:        pb.CopyLocation_LOG,
	CopyLocationKeccak:     pb.CopyLocation_KECCAK,
	CopyLocationReturnData: pb.CopyLocation_RETURNDATA,
}

var protoCopyLocationMap = common.ReverseMap(copyLocationToProtoMap)

func copyParticipantFromProto(participant *pb.CopyParticipant) CopyParticipant {
	ret := CopyParticipant{
		Location:   protoCopyLocationMap[participant.Location],
		MemAddress: participant.MemAddress,
	}
	switch id := participant.GetId().(type) {
	case *pb.CopyParticipant_CallId:
		txId := uint(id.CallId)
		ret.TxId = &txId
	case *pb.CopyParticipant_BytecodeHash:
		hash := common.HexToHash(id.BytecodeHash)
		ret.BytecodeHash = &hash
	case *pb.CopyParticipant_KeccakHash:
		hash := common.HexToHash(id.KeccakHash)
		ret.KeccakHash = &hash
	}
	return ret
}

func copyParticipantToProto(participant *CopyParticipant) *pb.CopyParticipant {
	ret := &pb.CopyParticipant{
		Location:   copyLocationToProtoMap[participant.Location],
		MemAddress: participant.MemAddress,
	}
	switch {
	case participant.TxId != nil:
		ret.Id = &pb.CopyParticipant_CallId{CallId: uint64(*participant.TxId)}
	case participant.BytecodeHash != nil:
		ret.Id = &pb.CopyParticipant_BytecodeHash{BytecodeHash: participant.BytecodeHash.Hex()}
	case participant.KeccakHash != nil:
		ret.Id = &pb.CopyParticipant_KeccakHash{KeccakHash: participant.KeccakHash.Hex()}
	}
	return ret
}

func mptTracesFromProto(pbMptTraces *pb.MPTTraces) (*mpttracer.MPTTraces, error) {
	check.PanicIfNot(pbMptTraces != nil)

	addrToStorageTraces := make(map[types.Address][]mpttracer.StorageTrieUpdateTrace, len(pbMptTraces.StorageTracesByAccount))
	for addr, pbStorageTrace := range pbMptTraces.StorageTracesByAccount {
		storageTraces := make([]mpttracer.StorageTrieUpdateTrace, len(pbStorageTrace.UpdatesTraces))
		for i, pbStorageUpdateTrace := range pbStorageTrace.UpdatesTraces {
			proof, err := protoToGoProof(pbStorageUpdateTrace.SszProof)
			if err != nil {
				return nil, err
			}
			storageTraces[i] = mpttracer.StorageTrieUpdateTrace{
				Key:         common.HexToHash(pbStorageUpdateTrace.Key),
				RootBefore:  common.HexToHash(pbStorageUpdateTrace.RootBefore),
				RootAfter:   common.HexToHash(pbStorageUpdateTrace.RootAfter),
				ValueBefore: protoUint256ToUint256(pbStorageUpdateTrace.ValueBefore),
				ValueAfter:  protoUint256ToUint256(pbStorageUpdateTrace.ValueAfter),
				Proof:       proof,
			}
		}
		addrToStorageTraces[types.HexToAddress(addr)] = storageTraces
	}

	contractTrieTraces := make([]mpttracer.ContractTrieUpdateTrace, len(pbMptTraces.ContractTrieTraces))
	for i, pbContractTrieUpdate := range pbMptTraces.ContractTrieTraces {
		proof, err := protoToGoProof(pbContractTrieUpdate.SszProof)
		if err != nil {
			return nil, err
		}
		contractTrieTraces[i] = mpttracer.ContractTrieUpdateTrace{
			Key:         common.HexToHash(pbContractTrieUpdate.Key),
			RootBefore:  common.HexToHash(pbContractTrieUpdate.RootBefore),
			RootAfter:   common.HexToHash(pbContractTrieUpdate.RootAfter),
			ValueBefore: smartContractFromProto(pbContractTrieUpdate.ValueBefore),
			ValueAfter:  smartContractFromProto(pbContractTrieUpdate.ValueAfter),
			Proof:       proof,
		}
	}

	ret := &mpttracer.MPTTraces{
		StorageTracesByAccount: addrToStorageTraces,
		ContractTrieTraces:     contractTrieTraces,
	}

	return ret, nil
}

func mptTracesToProto(mptTraces *mpttracer.MPTTraces) (*pb.MPTTraces, error) {
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
				Key:         storageUpdateTrace.Key.Hex(),
				RootBefore:  storageUpdateTrace.RootBefore.Hex(),
				RootAfter:   storageUpdateTrace.RootAfter.Hex(),
				ValueBefore: uint256ToProtoUint256(storageUpdateTrace.ValueBefore),
				ValueAfter:  uint256ToProtoUint256(storageUpdateTrace.ValueAfter),
				SszProof:    proof,
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
			Key:         contractTrieUpdate.Key.Hex(),
			RootBefore:  contractTrieUpdate.RootBefore.Hex(),
			RootAfter:   contractTrieUpdate.RootAfter.Hex(),
			ValueBefore: smartContractToProto(contractTrieUpdate.ValueBefore),
			ValueAfter:  smartContractToProto(contractTrieUpdate.ValueAfter),
			SszProof:    proof,
		}
	}

	ret := &pb.MPTTraces{
		StorageTracesByAccount: pbAddrToStorageTraces,
		ContractTrieTraces:     pbContractTrieTraces,
	}

	return ret, nil
}

// smartContractFromProto converts a Protocol Buffers SmartContract to Go SmartContract
func smartContractFromProto(pbSmartContract *pb.SmartContract) *types.SmartContract {
	if pbSmartContract == nil {
		return nil
	}

	var balance types.Value
	if pbSmartContract.Balance != nil {
		b := protoUint256ToUint256(pbSmartContract.Balance)
		balance = types.NewValue(b.Int())
	}
	return &types.SmartContract{
		Address:          types.HexToAddress(pbSmartContract.Address),
		Initialised:      pbSmartContract.Initialized,
		Balance:          types.NewValue(balance.Int()),
		CurrencyRoot:     common.HexToHash(pbSmartContract.CurrencyRoot),
		StorageRoot:      common.HexToHash(pbSmartContract.StorageRoot),
		CodeHash:         common.HexToHash(pbSmartContract.CodeHash),
		AsyncContextRoot: common.HexToHash(pbSmartContract.AsyncContextRoot),
		Seqno:            types.Seqno(pbSmartContract.Seqno),
		ExtSeqno:         types.Seqno(pbSmartContract.ExtSeqno),
		RequestId:        pbSmartContract.RequestId,
	}
}

// smartContractToProto converts a Go SmartContract to Protocol Buffers SmartContract
func smartContractToProto(smartContract *types.SmartContract) *pb.SmartContract {
	if smartContract == nil {
		return nil
	}

	var pbBalance *pb.Uint256
	if smartContract.Balance.Uint256 != nil {
		pbBalance = uint256ToProtoUint256(*smartContract.Balance.Uint256)
	}
	return &pb.SmartContract{
		Address:          smartContract.Address.Hex(),
		Initialized:      smartContract.Initialised,
		Balance:          pbBalance,
		CurrencyRoot:     smartContract.CurrencyRoot.Hex(),
		StorageRoot:      smartContract.StorageRoot.Hex(),
		CodeHash:         smartContract.CodeHash.Hex(),
		AsyncContextRoot: smartContract.AsyncContextRoot.Hex(),
		Seqno:            uint64(smartContract.Seqno),
		ExtSeqno:         uint64(smartContract.ExtSeqno),
		RequestId:        smartContract.RequestId,
	}
}
