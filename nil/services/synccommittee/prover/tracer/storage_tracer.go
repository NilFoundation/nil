package tracer

import (
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/tracing"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/internal/vm"
)

type StorageOp struct {
	IsRead    bool        // Is write otherwise
	Key       common.Hash // Key of element in storage
	Value     types.Uint256
	PrevValue types.Uint256
	PC        uint64
	TxnId     uint
	RwIdx     uint
	Addr      types.Address
}

type StateGetter interface {
	GetState(addr types.Address, key common.Hash) (common.Hash, error)
}

type StorageOpTracer struct {
	storageOps []StorageOp

	rwCtr       *RwCounter
	txnId       uint
	stateGetter StateGetter

	scope          tracing.OpContext
	prevOpFinisher func()
}

func NewStorageOpTracer(rwCtr *RwCounter, txnId uint, stateGetter StateGetter) *StorageOpTracer {
	return &StorageOpTracer{
		rwCtr:       rwCtr,
		txnId:       txnId,
		stateGetter: stateGetter,
	}
}

func (t *StorageOpTracer) GetStorageOps() []StorageOp {
	return t.storageOps
}

func (t *StorageOpTracer) TraceOp(opCode vm.OpCode, pc uint64, scope tracing.OpContext) (bool, error) {
	t.scope = scope
	//exhaustive:ignore only these to opcodes need to be handled
	switch opCode {
	case vm.SLOAD:
		stack := NewStackAccessor(scope.StackData())
		loc := stack.Pop()
		hash := common.Hash(loc.Bytes32())

		t.prevOpFinisher = func() {
			stack := NewStackAccessor(scope.StackData())
			value := stack.Pop()
			t.storageOps = append(t.storageOps, StorageOp{
				IsRead:    true,
				Key:       hash,
				Value:     types.Uint256(*value),
				PrevValue: types.Uint256(*value),
				PC:        pc,
				RwIdx:     t.rwCtr.NextIdx(),
				TxnId:     t.txnId,
				Addr:      scope.Address(),
			})
		}
	case vm.SSTORE:
		stack := NewStackAccessor(scope.StackData())
		loc := stack.Pop()
		value := stack.Pop()

		prevValue, err := t.stateGetter.GetState(scope.Address(), common.Hash(loc.Bytes32()))
		if err != nil {
			return false, err
		}

		t.storageOps = append(t.storageOps, StorageOp{
			IsRead:    false,
			Key:       common.Hash(loc.Bytes32()),
			Value:     types.Uint256(*value),
			PrevValue: types.Uint256(*prevValue.Uint256()),
			PC:        pc,
			RwIdx:     t.rwCtr.NextIdx(),
			TxnId:     t.txnId,
			Addr:      scope.Address(),
		})
	default:
		return false, nil
	}
	return true, nil
}

func (t *StorageOpTracer) FinishPrevOpcodeTracing() {
	if t.prevOpFinisher == nil {
		return
	}

	t.prevOpFinisher()
	t.prevOpFinisher = nil
}

func (t *StorageOpTracer) Finalize() []StorageOp {
	t.FinishPrevOpcodeTracing()
	return t.storageOps
}
