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
	bytecodeExtension = "bc"
	rwExtension       = "rw"
	zkevmExtension    = "zkevm"
	copyExtension     = "copy"
	mptExtension      = "mpt"
	expExtension      = "exp"
	keccakExtension   = "keccak"
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
		StackOps:          make([]StackOp, len(traces.rw.StackOps)),
		MemoryOps:         make([]MemoryOp, len(traces.rw.MemoryOps)),
		StorageOps:        make([]StorageOp, len(traces.rw.StorageOps)),
		ExpOps:            make([]ExpOp, len(traces.exp.ExpOps)),
		ZKEVMStates:       make([]ZKEVMState, len(traces.zkevm.ZkevmStates)),
		ContractsBytecode: make(map[types.Address][]byte, len(traces.bytecode.ContractBytecodes)),
		CopyEvents:        make([]CopyEvent, len(traces.copy.CopyEvents)),
		KeccakTraces:      make([]KeccakBuffer, len(traces.keccaks.HashedBuffers)),
	}

	for i, pbStackOp := range traces.rw.StackOps {
		ep.StackOps[i] = StackOp{
			IsRead: pbStackOp.IsRead,
			Idx:    int(pbStackOp.Index),
			Value:  *pb.ProtoUint256ToUint256(pbStackOp.Value),
			PC:     pbStackOp.Pc,
			TxnId:  uint(pbStackOp.TxnId),
			RwIdx:  uint(pbStackOp.RwIdx),
		}
	}

	for i, pbMemOp := range traces.rw.MemoryOps {
		ep.MemoryOps[i] = MemoryOp{
			IsRead: pbMemOp.IsRead,
			Idx:    int(pbMemOp.Index),
			Value:  pbMemOp.Value[0],
			PC:     pbMemOp.Pc,
			TxnId:  uint(pbMemOp.TxnId),
			RwIdx:  uint(pbMemOp.RwIdx),
		}
	}

	for i, pbStorageOp := range traces.rw.StorageOps {
		ep.StorageOps[i] = StorageOp{
			IsRead:    pbStorageOp.IsRead,
			Key:       common.HexToHash(pbStorageOp.Key),
			Value:     *pb.ProtoUint256ToUint256(pbStorageOp.Value),
			PrevValue: *pb.ProtoUint256ToUint256(pbStorageOp.PrevValue),
			PC:        pbStorageOp.Pc,
			TxnId:     uint(pbStorageOp.TxnId),
			RwIdx:     uint(pbStorageOp.RwIdx),
			Addr:      types.HexToAddress(pbStorageOp.Address.String()),
		}
	}

	for i, pbExpOp := range traces.exp.ExpOps {
		base := pb.ProtoUint256ToUint256(pbExpOp.Base)
		exponent := pb.ProtoUint256ToUint256(pbExpOp.Exponent)
		result := pb.ProtoUint256ToUint256(pbExpOp.Result)
		ep.ExpOps[i] = ExpOp{
			Base:     (*uint256.Int)(base),
			Exponent: (*uint256.Int)(exponent),
			Result:   (*uint256.Int)(result),
			PC:       pbExpOp.Pc,
			TxnId:    uint(pbExpOp.TxnId),
		}
	}

	for i, pbKeccakOp := range traces.keccaks.HashedBuffers {
		hash := pb.ProtoUint256ToUint256(pbKeccakOp.GetKeccakHash())
		ep.KeccakTraces[i] = KeccakBuffer{
			buf:  pbKeccakOp.GetBuffer(),
			hash: common.BytesToHash(hash.Bytes()),
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
			AdditionalInput: *pb.ProtoUint256ToUint256(pbZKEVMState.AdditionalInput),
			StackSize:       pbZKEVMState.StackSize,
			MemorySize:      pbZKEVMState.MemorySize,
			TxFinish:        pbZKEVMState.TxFinish,
			StackSlice:      make([]types.Uint256, len(pbZKEVMState.StackSlice)),
			MemorySlice:     make(map[uint64]uint8),
			StorageSlice:    make(map[types.Uint256]types.Uint256),
		}

		for j, stackVal := range pbZKEVMState.StackSlice {
			ep.ZKEVMStates[i].StackSlice[j] = *pb.ProtoUint256ToUint256(stackVal)
		}
		for addr, memVal := range pbZKEVMState.MemorySlice {
			ep.ZKEVMStates[i].MemorySlice[addr] = uint8(memVal)
		}
		for _, entry := range pbZKEVMState.StorageSlice {
			key := pb.ProtoUint256ToUint256(entry.Key)
			ep.ZKEVMStates[i].StorageSlice[*key] = *pb.ProtoUint256ToUint256(entry.Value)
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
