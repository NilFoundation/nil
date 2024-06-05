package execution

import (
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/mpt"
	"github.com/NilFoundation/nil/core/types"
	fastssz "github.com/ferranbt/fastssz"
	"github.com/holiman/uint256"
)

type MPTValue[S any] interface {
	~*S
	fastssz.Marshaler
	fastssz.Unmarshaler
}

type BaseMPT[K any, V any, VPtr MPTValue[V]] struct {
	*mpt.MerklePatriciaTrie

	keyToBytes   func(k K) []byte
	keyFromBytes func(bs []byte) (K, error)
}

type (
	ContractTrie    = BaseMPT[common.Hash, types.SmartContract, *types.SmartContract]
	MessageTrie     = BaseMPT[types.MessageIndex, types.Message, *types.Message]
	ReceiptTrie     = BaseMPT[types.MessageIndex, types.Receipt, *types.Receipt]
	StorageTrie     = BaseMPT[common.Hash, uint256.Int, *uint256.Int]
	ShardBlocksTrie = BaseMPT[types.ShardId, uint256.Int, *uint256.Int]
)

func NewContractTrie(parent *mpt.MerklePatriciaTrie) *ContractTrie {
	return newBaseMPT[common.Hash, types.SmartContract, *types.SmartContract](parent,
		func(k common.Hash) []byte { return k.Bytes() },
		func(bs []byte) (common.Hash, error) { return common.BytesToHash(bs), nil })
}

func NewMessageTrie(parent *mpt.MerklePatriciaTrie) *MessageTrie {
	return newBaseMPT[types.MessageIndex, types.Message, *types.Message](parent,
		func(k types.MessageIndex) []byte { return k.Bytes() },
		func(bs []byte) (types.MessageIndex, error) { return types.BytesToMessageIndex(bs), nil })
}

func NewReceiptTrie(parent *mpt.MerklePatriciaTrie) *ReceiptTrie {
	return newBaseMPT[types.MessageIndex, types.Receipt, *types.Receipt](parent,
		func(k types.MessageIndex) []byte { return k.Bytes() },
		func(bs []byte) (types.MessageIndex, error) { return types.BytesToMessageIndex(bs), nil })
}

func NewShardBlocksTrie(parent *mpt.MerklePatriciaTrie) *ShardBlocksTrie {
	return newBaseMPT[types.ShardId, uint256.Int, *uint256.Int](parent,
		func(k types.ShardId) []byte { return k.Bytes() },
		func(bs []byte) (types.ShardId, error) { return types.BytesToShardId(bs), nil })
}

func NewStorageTrie(parent *mpt.MerklePatriciaTrie) *StorageTrie {
	return newBaseMPT[common.Hash, uint256.Int, *uint256.Int](parent,
		func(k common.Hash) []byte { return k.Bytes() },
		func(bs []byte) (common.Hash, error) { return common.BytesToHash(bs), nil })
}

func newBaseMPT[K any, V any, VPtr MPTValue[V]](
	parent *mpt.MerklePatriciaTrie,
	keyToBytes func(k K) []byte,
	keyFromBytes func(bs []byte) (K, error),
) *BaseMPT[K, V, VPtr] {
	return &BaseMPT[K, V, VPtr]{
		MerklePatriciaTrie: parent,
		keyToBytes:         keyToBytes,
		keyFromBytes:       keyFromBytes,
	}
}

func (m *BaseMPT[K, V, VPtr]) newV() VPtr {
	var v V
	return VPtr(&v)
}

func (m *BaseMPT[K, V, VPtr]) Fetch(key K) (VPtr, error) {
	v := m.newV()
	raw, err := m.Get(m.keyToBytes(key))
	if err != nil {
		return nil, err
	}

	err = v.UnmarshalSSZ(raw)
	return v, err
}

func (m *BaseMPT[K, V, VPtr]) Update(key K, value VPtr) error {
	k := m.keyToBytes(key)
	v, err := value.MarshalSSZ()
	if err != nil {
		return err
	}

	return m.Set(k, v)
}

func (m *BaseMPT[K, V, VPtr]) Keys() ([]K, error) {
	// todo: choose good initial buffer size
	res := make([]K, 0, 100)
	for kv := range m.Iterate() {
		k, err := m.keyFromBytes(kv.Key)
		if err != nil {
			return nil, err
		}
		res = append(res, k)
	}
	return res, nil
}

func (m *BaseMPT[K, V, VPtr]) Values() ([]VPtr, error) {
	// todo: choose good initial buffer size
	res := make([]VPtr, 0, 100)
	for kv := range m.Iterate() {
		v := m.newV()
		if err := v.UnmarshalSSZ(kv.Value); err != nil {
			return nil, err
		}
		res = append(res, v)
	}
	return res, nil
}
