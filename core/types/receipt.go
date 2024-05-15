package types

import (
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/ssz"
)

type Receipts []*Receipt

type Receipt struct {
	Success bool
	GasUsed uint32
	Bloom   Bloom
	Logs    *ssz.ListSSZ[*Log]
}

var _ ssz.ObjectSSZ = (*Receipt)(nil)

func NewReceipt(success bool, gasUsed uint32) *Receipt {
	r := Receipt{Success: success, GasUsed: gasUsed}
	r.Logs = ssz.NewDynamicListSSZ[*Log](TopicsLimit)
	return &r
}

func (r *Receipt) EncodeSSZ(buf []byte) (res []byte, err error) {
	data, err := ssz.MarshalSSZ(buf, r.Success, &r.GasUsed, r.Bloom.Bytes(), r.Logs)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (r *Receipt) EncodingSizeSSZ() int {
	size := 0
	size += 1                            // Success
	size += 4                            // GasUsed
	size += BloomByteLength              // Bloom
	size += 4 + r.Logs.EncodingSizeSSZ() // Logs
	return size
}

func (r *Receipt) DecodeSSZ(buf []byte, version int) error {
	err := ssz.UnmarshalSSZ(buf, 0, &r.Success, &r.GasUsed, r.Bloom[:], r.Logs)
	if err != nil {
		return err
	}
	return nil
}

func (r *Receipt) Clone() common.Clonable {
	cloned := *r
	return &cloned
}
