package execution

import (
	fastssz "github.com/NilFoundation/fastssz"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/types"
)

type MPTValue[S any] interface {
	~*S
	fastssz.Marshaler
	fastssz.Unmarshaler
}

type Entry[K, V any] struct {
	Key K
	Val V
}

type BaseMPTReader[K any, V any, VPtr MPTValue[V]] struct {
	*mpt.Reader

	keyToBytes   func(k K) []byte
	keyFromBytes func(bs []byte) (K, error)
}

type BaseMPT[K any, V any, VPtr MPTValue[V]] struct {
	*BaseMPTReader[K, V, VPtr]

	rwTrie *mpt.MerklePatriciaTrie
}

type (
	ContractTrie     = BaseMPT[common.Hash, types.SmartContract, *types.SmartContract]
	MessageTrie      = BaseMPT[types.MessageIndex, types.Message, *types.Message]
	ReceiptTrie      = BaseMPT[types.MessageIndex, types.Receipt, *types.Receipt]
	StorageTrie      = BaseMPT[common.Hash, types.Uint256, *types.Uint256]
	CurrencyTrie     = BaseMPT[types.CurrencyId, types.Value, *types.Value]
	ShardBlocksTrie  = BaseMPT[types.ShardId, common.Hash, *common.Hash]
	AsyncContextTrie = BaseMPT[types.MessageIndex, types.AsyncContext, *types.AsyncContext]

	ContractTrieReader     = BaseMPTReader[common.Hash, types.SmartContract, *types.SmartContract]
	MessageTrieReader      = BaseMPTReader[types.MessageIndex, types.Message, *types.Message]
	ReceiptTrieReader      = BaseMPTReader[types.MessageIndex, types.Receipt, *types.Receipt]
	StorageTrieReader      = BaseMPTReader[common.Hash, types.Uint256, *types.Uint256]
	CurrencyTrieReader     = BaseMPTReader[types.CurrencyId, types.Value, *types.Value]
	ShardBlocksTrieReader  = BaseMPTReader[types.ShardId, common.Hash, *common.Hash]
	AsyncContextTrieReader = BaseMPTReader[types.MessageIndex, types.AsyncContext, *types.AsyncContext]
)

func NewContractTrieReader(parent *mpt.Reader) *ContractTrieReader {
	return &ContractTrieReader{
		parent,
		func(k common.Hash) []byte { return k.Bytes() },
		func(bs []byte) (common.Hash, error) { return common.BytesToHash(bs), nil },
	}
}

func NewMessageTrieReader(parent *mpt.Reader) *MessageTrieReader {
	return &MessageTrieReader{
		parent,
		func(k types.MessageIndex) []byte { return k.Bytes() },
		func(bs []byte) (types.MessageIndex, error) { return types.BytesToMessageIndex(bs), nil },
	}
}

func NewReceiptTrieReader(parent *mpt.Reader) *ReceiptTrieReader {
	return &ReceiptTrieReader{
		parent,
		func(k types.MessageIndex) []byte { return k.Bytes() },
		func(bs []byte) (types.MessageIndex, error) { return types.BytesToMessageIndex(bs), nil },
	}
}

func NewStorageTrieReader(parent *mpt.Reader) *StorageTrieReader {
	return &StorageTrieReader{
		parent,
		func(k common.Hash) []byte { return k.Bytes() },
		func(bs []byte) (common.Hash, error) { return common.BytesToHash(bs), nil },
	}
}

func NewAsyncContextTrieReader(parent *mpt.Reader) *AsyncContextTrieReader {
	return &AsyncContextTrieReader{
		parent,
		func(k types.MessageIndex) []byte { return k.Bytes() },
		func(bs []byte) (types.MessageIndex, error) { return types.BytesToMessageIndex(bs), nil },
	}
}

func NewCurrencyTrieReader(parent *mpt.Reader) *CurrencyTrieReader {
	return &CurrencyTrieReader{
		parent,
		func(k types.CurrencyId) []byte { return k[:] },
		func(bs []byte) (types.CurrencyId, error) {
			var res types.CurrencyId
			copy(res[:], bs)
			return res, nil
		},
	}
}

func NewShardBlocksTrieReader(parent *mpt.Reader) *ShardBlocksTrieReader {
	return &ShardBlocksTrieReader{
		parent,
		func(k types.ShardId) []byte { return k.Bytes() },
		func(bs []byte) (types.ShardId, error) { return types.BytesToShardId(bs), nil },
	}
}

func NewContractTrie(parent *mpt.MerklePatriciaTrie) *ContractTrie {
	return &ContractTrie{
		BaseMPTReader: NewContractTrieReader(parent.Reader),
		rwTrie:        parent,
	}
}

func NewMessageTrie(parent *mpt.MerklePatriciaTrie) *MessageTrie {
	return &MessageTrie{
		BaseMPTReader: NewMessageTrieReader(parent.Reader),
		rwTrie:        parent,
	}
}

func NewReceiptTrie(parent *mpt.MerklePatriciaTrie) *ReceiptTrie {
	return &ReceiptTrie{
		BaseMPTReader: NewReceiptTrieReader(parent.Reader),
		rwTrie:        parent,
	}
}

func NewStorageTrie(parent *mpt.MerklePatriciaTrie) *StorageTrie {
	return &StorageTrie{
		BaseMPTReader: NewStorageTrieReader(parent.Reader),
		rwTrie:        parent,
	}
}

func NewAsyncContextTrie(parent *mpt.MerklePatriciaTrie) *AsyncContextTrie {
	return &AsyncContextTrie{
		BaseMPTReader: NewAsyncContextTrieReader(parent.Reader),
		rwTrie:        parent,
	}
}

func NewCurrencyTrie(parent *mpt.MerklePatriciaTrie) *CurrencyTrie {
	return &CurrencyTrie{
		BaseMPTReader: NewCurrencyTrieReader(parent.Reader),
		rwTrie:        parent,
	}
}

func NewShardBlocksTrie(parent *mpt.MerklePatriciaTrie) *ShardBlocksTrie {
	return &ShardBlocksTrie{
		BaseMPTReader: NewShardBlocksTrieReader(parent.Reader),
		rwTrie:        parent,
	}
}

func NewDbContractTrieReader(tx db.RoTx, shardId types.ShardId) *ContractTrieReader {
	return NewContractTrieReader(mpt.NewDbReader(tx, shardId, db.ContractTrieTable))
}

func NewDbMessageTrieReader(tx db.RoTx, shardId types.ShardId) *MessageTrieReader {
	return NewMessageTrieReader(mpt.NewDbReader(tx, shardId, db.MessageTrieTable))
}

func NewDbReceiptTrieReader(tx db.RoTx, shardId types.ShardId) *ReceiptTrieReader {
	return NewReceiptTrieReader(mpt.NewDbReader(tx, shardId, db.ReceiptTrieTable))
}

func NewDbStorageTrieReader(tx db.RoTx, shardId types.ShardId) *StorageTrieReader {
	return NewStorageTrieReader(mpt.NewDbReader(tx, shardId, db.StorageTrieTable))
}

func NewDbCurrencyTrieReader(tx db.RoTx, shardId types.ShardId) *CurrencyTrieReader {
	return NewCurrencyTrieReader(mpt.NewDbReader(tx, shardId, db.CurrencyTrieTable))
}

func NewDbShardBlocksTrieReader(tx db.RoTx, shardId types.ShardId, blockNumber types.BlockNumber) *ShardBlocksTrieReader {
	return NewShardBlocksTrieReader(mpt.NewDbReader(tx, shardId, db.ShardBlocksTrieTableName(blockNumber)))
}

func NewDbContractTrie(tx db.RwTx, shardId types.ShardId) *ContractTrie {
	return NewContractTrie(mpt.NewDbMPT(tx, shardId, db.ContractTrieTable))
}

func NewDbMessageTrie(tx db.RwTx, shardId types.ShardId) *MessageTrie {
	return NewMessageTrie(mpt.NewDbMPT(tx, shardId, db.MessageTrieTable))
}

func NewDbReceiptTrie(tx db.RwTx, shardId types.ShardId) *ReceiptTrie {
	return NewReceiptTrie(mpt.NewDbMPT(tx, shardId, db.ReceiptTrieTable))
}

func NewDbStorageTrie(tx db.RwTx, shardId types.ShardId) *StorageTrie {
	return NewStorageTrie(mpt.NewDbMPT(tx, shardId, db.StorageTrieTable))
}

func NewDbAsyncContextTrie(tx db.RwTx, shardId types.ShardId) *AsyncContextTrie {
	return NewAsyncContextTrie(mpt.NewDbMPT(tx, shardId, db.AsyncCallContextTable))
}

func NewDbCurrencyTrie(tx db.RwTx, shardId types.ShardId) *CurrencyTrie {
	return NewCurrencyTrie(mpt.NewDbMPT(tx, shardId, db.CurrencyTrieTable))
}

func NewDbShardBlocksTrie(tx db.RwTx, shardId types.ShardId, blockNumber types.BlockNumber) *ShardBlocksTrie {
	return NewShardBlocksTrie(mpt.NewDbMPT(tx, shardId, db.ShardBlocksTrieTableName(blockNumber)))
}

func (m *BaseMPTReader[K, V, VPtr]) newV() VPtr {
	var v V
	return VPtr(&v)
}

func (m *BaseMPTReader[K, V, VPtr]) Fetch(key K) (VPtr, error) {
	v := m.newV()
	raw, err := m.Get(m.keyToBytes(key))
	if err != nil {
		return nil, err
	}

	err = v.UnmarshalSSZ(raw)
	return v, err
}

func (m *BaseMPTReader[K, V, VPtr]) Entries() ([]Entry[K, VPtr], error) {
	res := make([]Entry[K, VPtr], 0)
	for key, value := range m.Iterate() {
		k, err := m.keyFromBytes(key)
		if err != nil {
			return nil, err
		}

		v := m.newV()
		if err := v.UnmarshalSSZ(value); err != nil {
			return nil, err
		}

		res = append(res, Entry[K, VPtr]{k, v})
	}
	return res, nil
}

func (m *BaseMPTReader[K, V, VPtr]) Keys() ([]K, error) {
	res := make([]K, 0)
	for key := range m.Iterate() {
		k, err := m.keyFromBytes(key)
		if err != nil {
			return nil, err
		}
		res = append(res, k)
	}
	return res, nil
}

func (m *BaseMPTReader[K, V, VPtr]) Values() ([]VPtr, error) {
	res := make([]VPtr, 0)
	for _, value := range m.Iterate() {
		v := m.newV()
		if err := v.UnmarshalSSZ(value); err != nil {
			return nil, err
		}
		res = append(res, v)
	}
	return res, nil
}

func (m *BaseMPT[K, V, VPtr]) Update(key K, value VPtr) error {
	k := m.keyToBytes(key)
	v, err := value.MarshalSSZ()
	if err != nil {
		return err
	}

	return m.rwTrie.Set(k, v)
}
