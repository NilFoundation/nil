package tracer

import (
	"crypto/rand"
	"fmt"
	"math"
	"math/big"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/internal/vm"
	pb "github.com/NilFoundation/nil/nil/services/synccommittee/prover/proto"
	"github.com/NilFoundation/nil/nil/services/synccommittee/prover/tracer/internal/constants"
	"github.com/NilFoundation/nil/nil/services/synccommittee/prover/tracer/internal/mpttracer"
	"github.com/holiman/uint256"
	"golang.org/x/sync/errgroup"
)

// Set of pb messages split by circuits
type PbTracesSet struct {
	bytecode *pb.BytecodeTraces
	rw       *pb.RWTraces
	zkevm    *pb.ZKEVMTraces
	copy     *pb.CopyTraces
	mpt      *pb.MPTTraces
	exp      *pb.ExpTraces
	keccaks  *pb.KeccakTraces
}

// Each message is serialized into file with corresponding extension added to base file path
const (
	bytecodeExtension  = "bc"
	rwExtension        = "rw"
	zkevmExtension     = "zkevm"
	copyExtension      = "copy"
	mptExtension       = "mpt"
	expExtension       = "exp"
	keccakExtension    = "keccak"
	zethCacheExtension = "zeth"
)

func SerializeToFile(proofs *ExecutionTraces, mode MarshalMode, baseFileName string) error {
	randval, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		return err
	}

	// Convert ExecutionTraces to protobuf messages set
	pbTraces, err := ToProto(proofs, uint64(randval.Int64()))
	if err != nil {
		return err
	}

	// Write trace files in parallel
	eg := errgroup.Group{} // TODO: use WithContext to cancel remaining jobs in case of error

	marshalModes := mode.getMarshallers()
	for ext, marshalFunc := range marshalModes {
		// Marshal zkevm message
		eg.Go(func() error {
			return marshalToFile(pbTraces.zkevm,
				marshalFunc, fmt.Sprintf("%s.%s.%s", baseFileName, zkevmExtension, ext))
		})

		// Marshal bytecode message
		eg.Go(func() error {
			return marshalToFile(pbTraces.bytecode,
				marshalFunc, fmt.Sprintf("%s.%s.%s", baseFileName, bytecodeExtension, ext))
		})

		// Marshal rw message
		eg.Go(func() error {
			return marshalToFile(pbTraces.rw,
				marshalFunc, fmt.Sprintf("%s.%s.%s", baseFileName, rwExtension, ext))
		})

		// Marshal copy message
		eg.Go(func() error {
			return marshalToFile(pbTraces.copy,
				marshalFunc, fmt.Sprintf("%s.%s.%s", baseFileName, copyExtension, ext))
		})

		// Marshal mpt traces message
		eg.Go(func() error {
			return marshalToFile(pbTraces.mpt,
				marshalFunc, fmt.Sprintf("%s.%s.%s", baseFileName, mptExtension, ext))
		})

		// Marshal exp traces message
		eg.Go(func() error {
			return marshalToFile(pbTraces.exp,
				marshalFunc, fmt.Sprintf("%s.%s.%s", baseFileName, expExtension, ext))
		})

		eg.Go(func() error {
			return marshalToFile(pbTraces.keccaks,
				marshalFunc, fmt.Sprintf("%s.%s.%s", baseFileName, keccakExtension, ext))
		})
	}

	eg.Go(func() error {
		return marshalJSONToFile(
			proofs.ZethCache,
			fmt.Sprintf("%s.%s.%s", baseFileName, zethCacheExtension, MarshalModeJSON),
		)
	})

	return eg.Wait()
}

func DeserializeFromFile(baseFileName string, mode MarshalMode) (*ExecutionTraces, error) {
	pbTraces := PbTracesSet{
		bytecode: &pb.BytecodeTraces{},
		rw:       &pb.RWTraces{},
		zkevm:    &pb.ZKEVMTraces{},
		copy:     &pb.CopyTraces{},
		mpt:      &pb.MPTTraces{},
		exp:      &pb.ExpTraces{},
		keccaks:  &pb.KeccakTraces{},
	}

	unmarshal, ok := marshalModeToUnmarshaller[mode]
	if !ok {
		return nil, fmt.Errorf("no unmarshaler found for mode %d", mode)
	}

	ext := mode.String()

	// Unmarshal trace files in parallel
	eg := errgroup.Group{}

	// Unmarshal zkevm message
	eg.Go(func() error {
		return unmarshalFromFile(fmt.Sprintf("%s.%s.%s", baseFileName, zkevmExtension, ext),
			unmarshal, pbTraces.zkevm)
	})

	// Unmarshal bc message
	eg.Go(func() error {
		return unmarshalFromFile(fmt.Sprintf("%s.%s.%s", baseFileName, bytecodeExtension, ext),
			unmarshal, pbTraces.bytecode)
	})

	// Unmarshal rw message
	eg.Go(func() error {
		return unmarshalFromFile(fmt.Sprintf("%s.%s.%s", baseFileName, rwExtension, ext),
			unmarshal, pbTraces.rw)
	})

	// Unmarshal copy message
	eg.Go(func() error {
		return unmarshalFromFile(fmt.Sprintf("%s.%s.%s", baseFileName, copyExtension, ext),
			unmarshal, pbTraces.copy)
	})

	// Unmarshal mpt traces message
	eg.Go(func() error {
		return unmarshalFromFile(fmt.Sprintf("%s.%s.%s", baseFileName, mptExtension, ext),
			unmarshal, pbTraces.mpt)
	})

	// Unmarshal exp traces message
	eg.Go(func() error {
		return unmarshalFromFile(fmt.Sprintf("%s.%s.%s", baseFileName, expExtension, ext),
			unmarshal, pbTraces.exp)
	})

	eg.Go(func() error {
		return unmarshalFromFile(fmt.Sprintf("%s.%s.%s", baseFileName, keccakExtension, ext),
			unmarshal, pbTraces.keccaks)
	})

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	// Convert protobuf messages back to ExecutionTraces
	return FromProto(&pbTraces)
}

func FromProto(traces *PbTracesSet) (*ExecutionTraces, error) {
	ep := &ExecutionTraces{
		StackOps:          make([]StackOp, len(traces.rw.GetStackOps())),
		MemoryOps:         make([]MemoryOp, len(traces.rw.GetMemoryOps())),
		StorageOps:        make([]StorageOp, len(traces.rw.GetStorageOps())),
		ExpOps:            make([]ExpOp, len(traces.exp.GetExpOps())),
		ZKEVMStates:       make([]ZKEVMState, len(traces.zkevm.GetZkevmStates())),
		ContractsBytecode: make(map[types.Address][]byte, len(traces.bytecode.GetContractBytecodes())),
		CopyEvents:        make([]CopyEvent, len(traces.copy.GetCopyEvents())),
		KeccakTraces:      make([]KeccakBuffer, len(traces.keccaks.GetHashedBuffers())),
	}

	for i, pbStackOp := range traces.rw.GetStackOps() {
		ep.StackOps[i] = StackOp{
			IsRead: pbStackOp.GetIsRead(),
			Idx:    int(pbStackOp.GetIndex()),
			Value:  *pb.ProtoUint256ToUint256(pbStackOp.GetValue()),
			PC:     pbStackOp.GetPc(),
			TxnId:  uint(pbStackOp.GetTxnId()),
			RwIdx:  uint(pbStackOp.GetRwIdx()),
		}
	}

	for i, pbMemOp := range traces.rw.GetMemoryOps() {
		ep.MemoryOps[i] = MemoryOp{
			IsRead: pbMemOp.GetIsRead(),
			Idx:    int(pbMemOp.GetIndex()),
			Value:  pbMemOp.GetValue()[0],
			PC:     pbMemOp.GetPc(),
			TxnId:  uint(pbMemOp.GetTxnId()),
			RwIdx:  uint(pbMemOp.GetRwIdx()),
		}
	}

	for i, pbStorageOp := range traces.rw.GetStorageOps() {
		ep.StorageOps[i] = StorageOp{
			IsRead:    pbStorageOp.GetIsRead(),
			Key:       common.HexToHash(pbStorageOp.GetKey()),
			Value:     *pb.ProtoUint256ToUint256(pbStorageOp.GetValue()),
			PrevValue: *pb.ProtoUint256ToUint256(pbStorageOp.GetPrevValue()),
			PC:        pbStorageOp.GetPc(),
			TxnId:     uint(pbStorageOp.GetTxnId()),
			RwIdx:     uint(pbStorageOp.GetRwIdx()),
			Addr:      types.HexToAddress(pbStorageOp.GetAddress().String()),
		}
	}

	for i, pbExpOp := range traces.exp.GetExpOps() {
		base := pb.ProtoUint256ToUint256(pbExpOp.GetBase())
		exponent := pb.ProtoUint256ToUint256(pbExpOp.GetExponent())
		result := pb.ProtoUint256ToUint256(pbExpOp.GetResult())
		ep.ExpOps[i] = ExpOp{
			Base:     (*uint256.Int)(base),
			Exponent: (*uint256.Int)(exponent),
			Result:   (*uint256.Int)(result),
			PC:       pbExpOp.GetPc(),
			TxnId:    uint(pbExpOp.GetTxnId()),
		}
	}

	for i, pbKeccakOp := range traces.keccaks.GetHashedBuffers() {
		hash := pb.ProtoUint256ToUint256(pbKeccakOp.GetKeccakHash())
		ep.KeccakTraces[i] = KeccakBuffer{
			buf:  pbKeccakOp.GetBuffer(),
			hash: common.BytesToHash(hash.Bytes()),
		}
	}

	for i, pbZKEVMState := range traces.zkevm.GetZkevmStates() {
		ep.ZKEVMStates[i] = ZKEVMState{
			TxHash:          common.HexToHash(pbZKEVMState.GetTxHash()),
			TxId:            int(pbZKEVMState.GetCallId()),
			PC:              pbZKEVMState.GetPc(),
			Gas:             pbZKEVMState.GetGas(),
			RwIdx:           uint(pbZKEVMState.GetRwIdx()),
			BytecodeHash:    common.HexToHash(pbZKEVMState.GetBytecodeHash()),
			OpCode:          vm.OpCode(pbZKEVMState.GetOpcode()),
			AdditionalInput: *pb.ProtoUint256ToUint256(pbZKEVMState.GetAdditionalInput()),
			StackSize:       pbZKEVMState.GetStackSize(),
			MemorySize:      pbZKEVMState.GetMemorySize(),
			TxFinish:        pbZKEVMState.GetTxFinish(),
			StackSlice:      make([]types.Uint256, len(pbZKEVMState.GetStackSlice())),
			MemorySlice:     make(map[uint64]uint8),
			StorageSlice:    make(map[types.Uint256]types.Uint256),
		}

		for j, stackVal := range pbZKEVMState.GetStackSlice() {
			ep.ZKEVMStates[i].StackSlice[j] = *pb.ProtoUint256ToUint256(stackVal)
		}
		for addr, memVal := range pbZKEVMState.GetMemorySlice() {
			ep.ZKEVMStates[i].MemorySlice[addr] = uint8(memVal)
		}
		for _, entry := range pbZKEVMState.GetStorageSlice() {
			key := pb.ProtoUint256ToUint256(entry.GetKey())
			ep.ZKEVMStates[i].StorageSlice[*key] = *pb.ProtoUint256ToUint256(entry.GetValue())
		}
	}

	for i, pbCopyEventTrace := range traces.copy.GetCopyEvents() {
		ep.CopyEvents[i].From = copyParticipantFromProto(pbCopyEventTrace.GetFrom())
		ep.CopyEvents[i].To = copyParticipantFromProto(pbCopyEventTrace.GetTo())
		ep.CopyEvents[i].RwIdx = uint(pbCopyEventTrace.GetRwIdx())
		ep.CopyEvents[i].Data = pbCopyEventTrace.GetData()
	}

	for pbContractAddr, pbContractBytecode := range traces.bytecode.GetContractBytecodes() {
		ep.ContractsBytecode[types.HexToAddress(pbContractAddr)] = pbContractBytecode
	}

	mptTraces, err := mpttracer.TracesFromProto(traces.mpt)
	if err != nil {
		return nil, err
	}
	ep.MPTTraces = mptTraces

	return ep, nil
}

func ToProto(traces *ExecutionTraces, traceIdx uint64) (*PbTracesSet, error) {
	pbTraces := &PbTracesSet{
		bytecode: &pb.BytecodeTraces{
			ContractBytecodes: make(map[string][]byte),
			TraceIdx:          traceIdx,
			ProtoHash:         constants.ProtoHash,
		},
		rw: &pb.RWTraces{
			StackOps:   make([]*pb.StackOp, len(traces.StackOps)),
			MemoryOps:  make([]*pb.MemoryOp, len(traces.MemoryOps)),
			StorageOps: make([]*pb.StorageOp, len(traces.StorageOps)),
			TraceIdx:   traceIdx,
			ProtoHash:  constants.ProtoHash,
		},
		exp: &pb.ExpTraces{
			ExpOps:    make([]*pb.ExpOp, len(traces.ExpOps)),
			TraceIdx:  traceIdx,
			ProtoHash: constants.ProtoHash,
		},
		zkevm: &pb.ZKEVMTraces{
			ZkevmStates: make([]*pb.ZKEVMState, len(traces.ZKEVMStates)),
			TraceIdx:    traceIdx,
			ProtoHash:   constants.ProtoHash,
		},
		copy: &pb.CopyTraces{
			CopyEvents: make([]*pb.CopyEvent, len(traces.CopyEvents)),
			TraceIdx:   traceIdx,
			ProtoHash:  constants.ProtoHash,
		},
		keccaks: &pb.KeccakTraces{
			HashedBuffers: make([]*pb.KeccakBuffer, len(traces.KeccakTraces)),
			TraceIdx:      traceIdx,
			ProtoHash:     constants.ProtoHash,
		},
	}

	// Convert StackOps
	for i, stackOp := range traces.StackOps {
		pbTraces.rw.StackOps[i] = &pb.StackOp{
			IsRead: stackOp.IsRead,
			Index:  int32(stackOp.Idx),
			Value:  pb.Uint256ToProtoUint256(&stackOp.Value),
			Pc:     stackOp.PC,
			TxnId:  uint64(stackOp.TxnId),
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
			TxnId:  uint64(memOp.TxnId),
			RwIdx:  uint64(memOp.RwIdx),
		}
	}

	// Convert StorageOps
	for i, storageOp := range traces.StorageOps {
		pbTraces.rw.StorageOps[i] = &pb.StorageOp{
			IsRead:    storageOp.IsRead,
			Key:       storageOp.Key.Hex(),
			Value:     pb.Uint256ToProtoUint256(&storageOp.Value),
			PrevValue: pb.Uint256ToProtoUint256(&storageOp.PrevValue),
			Pc:        storageOp.PC,
			TxnId:     uint64(storageOp.TxnId),
			RwIdx:     uint64(storageOp.RwIdx),
			Address:   &pb.Address{AddressBytes: storageOp.Addr.Bytes()},
		}
	}

	for i, expOp := range traces.ExpOps {
		pbTraces.exp.ExpOps[i] = &pb.ExpOp{
			Base:     pb.Uint256ToProtoUint256((*types.Uint256)(expOp.Base)),
			Exponent: pb.Uint256ToProtoUint256((*types.Uint256)(expOp.Exponent)),
			Result:   pb.Uint256ToProtoUint256((*types.Uint256)(expOp.Result)),
			Pc:       expOp.PC,
			TxnId:    uint64(expOp.TxnId),
		}
	}

	for i, keccakOp := range traces.KeccakTraces {
		hash := keccakOp.hash.Uint256()
		pbTraces.keccaks.HashedBuffers[i] = &pb.KeccakBuffer{
			Buffer:     keccakOp.buf,
			KeccakHash: pb.Uint256ToProtoUint256((*types.Uint256)(hash)),
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
			AdditionalInput: pb.Uint256ToProtoUint256(&zkevmState.AdditionalInput),
			StackSize:       zkevmState.StackSize,
			MemorySize:      zkevmState.MemorySize,
			TxFinish:        zkevmState.TxFinish,
			StackSlice:      make([]*pb.Uint256, len(zkevmState.StackSlice)),
			MemorySlice:     make(map[uint64]uint32),
			StorageSlice:    make([]*pb.StorageEntry, len(zkevmState.StorageSlice)),
		}
		for j, stackVal := range zkevmState.StackSlice {
			pbTraces.zkevm.ZkevmStates[i].StackSlice[j] = pb.Uint256ToProtoUint256(&stackVal)
		}
		for addr, memVal := range zkevmState.MemorySlice {
			pbTraces.zkevm.ZkevmStates[i].MemorySlice[addr] = uint32(memVal)
		}
		storageSliceCounter := 0
		for storageKey, storageVal := range zkevmState.StorageSlice {
			pbEntry := &pb.StorageEntry{
				Key:   pb.Uint256ToProtoUint256(&storageKey),
				Value: pb.Uint256ToProtoUint256(&storageVal),
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

	mptTraces, err := mpttracer.TracesToProto(traces.MPTTraces, traceIdx)
	if err != nil {
		return nil, err
	}
	pbTraces.mpt = mptTraces

	return pbTraces, nil
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
		Location:   protoCopyLocationMap[participant.GetLocation()],
		MemAddress: participant.GetMemAddress(),
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
