package tracer

import (
	"errors"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
)

type StorageOp struct {
	IsRead  bool        // Is write otherwise
	Address common.Hash // Key of element in storage
	Value   *types.Uint256
	PC      uint64
	MsgId   uint
	RwIdx   uint
}

type StateSlotDiff struct {
	before *types.Uint256
	after  *types.Uint256
}

type MessageStorageInteractor struct {
	// Within a single message processing mutliple accounts states modification could occur
	msgChangedStateSlots map[types.Address]map[common.Hash]StateSlotDiff

	storageOps []StorageOp
	rwCtr      *RwCounter
	pcGetter   func() uint64
	msgId      uint
}

func NewMessageStorageInteractor(rwCtr *RwCounter, pcGetter func() uint64, msgId uint) *MessageStorageInteractor {
	return &MessageStorageInteractor{
		rwCtr:                rwCtr,
		pcGetter:             pcGetter,
		msgId:                msgId,
		msgChangedStateSlots: make(map[types.Address]map[common.Hash]StateSlotDiff),
	}
}

func (d *MessageStorageInteractor) GetStorageOps() []StorageOp {
	return d.storageOps
}

func (d *MessageStorageInteractor) GetSlot(acc *Account, key common.Hash) (common.Hash, error) {
	accountSlots, _ := d.msgChangedStateSlots[acc.Address]
	slotDiff, exists := accountSlots[key]
	if exists {
		d.storageOps = append(d.storageOps, StorageOp{
			IsRead:  true,
			Address: key,
			Value:   slotDiff.after,
			PC:      d.pcGetter(),
			RwIdx:   d.rwCtr.NextIdx(),
			MsgId:   d.msgId,
		})
		return slotDiff.after.Bytes32(), nil
	}

	// If wasn't modified, read from db
	val, err := d.readFromDb(acc, key)
	if err != nil {
		d.storageOps = append(d.storageOps, StorageOp{
			IsRead:  true,
			Address: key,
			Value:   (*types.Uint256)(val.Uint256()),
			PC:      d.pcGetter(),
			RwIdx:   d.rwCtr.NextIdx(),
			MsgId:   d.msgId,
		})
	}
	return val, err
}

func (d *MessageStorageInteractor) readFromDb(acc *Account, key common.Hash) (common.Hash, error) {
	ret, err := acc.StorageTrie.Fetch(key)
	if errors.Is(err, db.ErrKeyNotFound) {
		return common.EmptyHash, nil
	}
	if err != nil {
		return common.EmptyHash, err
	}
	return ret.Bytes32(), err
}

// During processing a single message we need to track only initial and final value for each changed slot.
// Do not modify trie before processing is done, just keep the changes in a map. After message is processed,
// apply each slot modification, for each modification provide an MPT proof.
func (d *MessageStorageInteractor) SetSlot(acc *Account, key common.Hash, val common.Hash) error {
	accountSlots, _ := d.msgChangedStateSlots[acc.Address]
	slotDiff, exists := accountSlots[key]
	if !exists {
		initialVal, err := d.readFromDb(acc, key)
		if err != nil {
			return err
		}

		slotDiff.before = (*types.Uint256)(initialVal.Uint256())
	}

	uintVal := (*types.Uint256)(val.Uint256())
	slotDiff.after = uintVal

	err := acc.StorageTrie.Update(key, uintVal)
	if err != nil {
		return err
	}

	slotDiff.after = uintVal

	d.storageOps = append(d.storageOps, StorageOp{
		IsRead:  false,
		Address: key,
		Value:   uintVal,
		PC:      d.pcGetter(),
		RwIdx:   d.rwCtr.NextIdx(),
		MsgId:   d.msgId,
	})

	return nil
}

type SlotChangeTrace struct {
	Key         common.Hash
	RootBefore  common.Hash
	RootAfter   common.Hash
	ValueBefore *types.Uint256
	ValueAfter  *types.Uint256
	Proof       mpt.Proof
}

func (d *MessageStorageInteractor) GetAccountSlotChangeTraces(acc *Account) ([]SlotChangeTrace, error) {
	accStateChanges, exists := d.msgChangedStateSlots[acc.Address]
	if !exists {
		// No state changes were caught for this address
		return nil, nil
	}

	traces := make([]SlotChangeTrace, len(accStateChanges))
	for storageKey, diff := range accStateChanges {
		var err error
		slotChangeTrace := SlotChangeTrace{
			RootBefore:  acc.StorageTrie.RootHash(),
			ValueBefore: diff.before,
		}

		if err = acc.StorageTrie.Update(storageKey, diff.after); err != nil {
			return nil, err
		}

		slotChangeTrace.RootAfter = acc.StorageTrie.RootHash()
		slotChangeTrace.ValueAfter = diff.after

		slotChangeTrace.Proof, err = mpt.BuildProof(acc.StorageTrie.Reader, storageKey.Bytes(), mpt.SetMPTOperation)
		if err != nil {
			return nil, err
		}

		traces = append(traces, slotChangeTrace)
	}

	return traces, nil
}

func (d *MessageStorageInteractor) GetAffectedAccountsAddresses() []types.Address {
	addresses := make([]types.Address, len(d.msgChangedStateSlots))
	for add := range d.msgChangedStateSlots {
		addresses = append(addresses, add)
	}
	return addresses
}
