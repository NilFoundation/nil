package tracer

import (
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/types"
)

type StorageOp struct {
	IsRead    bool        // Is write otherwise
	Key       common.Hash // Key of element in storage
	Value     types.Uint256
	PrevValue types.Uint256
	PC        uint64
	MsgId     uint
	RwIdx     uint
	Addr      types.Address
}

type StorageOpTracer struct {
	mptTracer  SlotStorage
	storageOps []StorageOp
	rwCtr      *RwCounter
	pcGetter   func() uint64
	msgId      uint
}

type SlotStorage interface {
	GetSlot(addr types.Address, key common.Hash) (common.Hash, error)
	SetSlot(addr types.Address, key common.Hash, value common.Hash) error
}

func NewStorageOpTracer(mptTracer SlotStorage, rwCtr *RwCounter, pcGetter func() uint64, msgId uint) *StorageOpTracer {
	return &StorageOpTracer{
		mptTracer: mptTracer,
		rwCtr:     rwCtr,
		pcGetter:  pcGetter,
		msgId:     msgId,
	}
}

func (d *StorageOpTracer) GetStorageOps() []StorageOp {
	return d.storageOps
}

func (d *StorageOpTracer) GetSlot(addr types.Address, key common.Hash) (common.Hash, error) {
	// `mptTracer.GetSlot` returns `nil, nil` in case of no such addr exists.
	// Such read operation will be also included into traces.
	value, err := d.mptTracer.GetSlot(addr, key)
	if err != nil {
		return common.EmptyHash, err
	}
	d.storageOps = append(d.storageOps, StorageOp{
		IsRead:    true,
		Key:       key,
		Value:     types.Uint256(*value.Uint256()),
		PrevValue: types.Uint256(*value.Uint256()),
		PC:        d.pcGetter(),
		RwIdx:     d.rwCtr.NextIdx(),
		MsgId:     d.msgId,
		Addr:      addr,
	})

	return value, err
}

// Calls `SetSlot` of MPTTracer, saves StorageOP. Returns previous slot value.
func (d *StorageOpTracer) SetSlot(addr types.Address, key common.Hash, val common.Hash) (common.Hash, error) {
	prevVal, err := d.mptTracer.GetSlot(addr, key)
	if err != nil {
		return common.EmptyHash, err
	}

	err = d.mptTracer.SetSlot(addr, key, val)
	if err != nil {
		return common.EmptyHash, err
	}

	d.storageOps = append(d.storageOps, StorageOp{
		IsRead:    false,
		Key:       key,
		Value:     types.Uint256(*val.Uint256()),
		PrevValue: types.Uint256(*prevVal.Uint256()),
		PC:        d.pcGetter(),
		RwIdx:     d.rwCtr.NextIdx(),
		MsgId:     d.msgId,
		Addr:      addr,
	})

	return prevVal, nil
}
