package execution

import (
	"iter"

	fastssz "github.com/NilFoundation/fastssz"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
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
	Key K `json:"k"`
	Val V `json:"v"`
}

type BaseMPTReader[K any, V any, VPtr MPTValue[V]] struct {
	*mpt.Reader

	keyToBytes   func(k K) []byte
	keyFromBytes func(bs []byte) *K // returns nil if the bytes do not encode a key
}

type BaseMPT[K any, V any, VPtr MPTValue[V]] struct {
	*BaseMPTReader[K, V, VPtr]

	rwTrie *mpt.MerklePatriciaTrie
}

type (
	ContractTrie     = BaseMPT[common.Hash, types.SmartContract, *types.SmartContract]
	TransactionTrie  = BaseMPT[types.TransactionIndex, types.Transaction, *types.Transaction]
	TxCountTrie      = BaseMPT[types.ShardId, types.TransactionIndex, *types.TransactionIndex]
	ReceiptTrie      = BaseMPT[types.TransactionIndex, types.Receipt, *types.Receipt]
	StorageTrie      = BaseMPT[common.Hash, types.Uint256, *types.Uint256]
	TokenTrie        = BaseMPT[types.TokenId, types.Value, *types.Value]
	ShardBlocksTrie  = BaseMPT[types.ShardId, common.Hash, *common.Hash]
	AsyncContextTrie = BaseMPT[types.TransactionIndex, types.AsyncContext, *types.AsyncContext]

	ContractTrieReader     = BaseMPTReader[common.Hash, types.SmartContract, *types.SmartContract]
	TransactionTrieReader  = BaseMPTReader[types.TransactionIndex, types.Transaction, *types.Transaction]
	TxCountTrieReader      = BaseMPTReader[types.ShardId, types.TransactionIndex, *types.TransactionIndex]
	ReceiptTrieReader      = BaseMPTReader[types.TransactionIndex, types.Receipt, *types.Receipt]
	StorageTrieReader      = BaseMPTReader[common.Hash, types.Uint256, *types.Uint256]
	TokenTrieReader        = BaseMPTReader[types.TokenId, types.Value, *types.Value]
	ShardBlocksTrieReader  = BaseMPTReader[types.ShardId, common.Hash, *common.Hash]
	AsyncContextTrieReader = BaseMPTReader[types.TransactionIndex, types.AsyncContext, *types.AsyncContext]
)

func NewContractTrieReader(parent *mpt.Reader) *ContractTrieReader {
	return &ContractTrieReader{
		parent,
		func(k common.Hash) []byte { return k.Bytes() },
		func(bs []byte) *common.Hash { h := common.BytesToHash(bs); return &h },
	}
}

func newTransactionTrieReader(parent *mpt.Reader) *TransactionTrieReader {
	return &TransactionTrieReader{
		parent,
		func(k types.TransactionIndex) []byte { return k.Bytes() },
		func(bs []byte) *types.TransactionIndex {
			// The key is decodable as a TransactionIndex iff it has the size of a TransactionIndex
			if len(bs) != types.TransactionIndexSize {
				return nil
			}
			idx := types.BytesToTransactionIndex(bs)
			return &idx
		},
	}
}

func newTxCountTrieReader(parent *mpt.Reader) *TxCountTrieReader {
	return &TxCountTrieReader{
		parent,
		func(k types.ShardId) []byte { return k.Bytes() },
		func(bs []byte) *types.ShardId {
			// The key is decodable as a ShardId iff it has the size of a ShardId
			if len(bs) != types.ShardIdSize {
				return nil
			}
			shard := types.BytesToShardId(bs)
			return &shard
		},
	}
}

func NewReceiptTrieReader(parent *mpt.Reader) *ReceiptTrieReader {
	return &ReceiptTrieReader{
		parent,
		func(k types.TransactionIndex) []byte { return k.Bytes() },
		func(bs []byte) *types.TransactionIndex { idx := types.BytesToTransactionIndex(bs); return &idx },
	}
}

func NewStorageTrieReader(parent *mpt.Reader) *StorageTrieReader {
	return &StorageTrieReader{
		parent,
		func(k common.Hash) []byte { return k.Bytes() },
		func(bs []byte) *common.Hash { h := common.BytesToHash(bs); return &h },
	}
}

func newAsyncContextTrieReader(parent *mpt.Reader) *AsyncContextTrieReader {
	return &AsyncContextTrieReader{
		parent,
		func(k types.TransactionIndex) []byte { return k.Bytes() },
		func(bs []byte) *types.TransactionIndex { idx := types.BytesToTransactionIndex(bs); return &idx },
	}
}

func NewTokenTrieReader(parent *mpt.Reader) *TokenTrieReader {
	return &TokenTrieReader{
		parent,
		func(k types.TokenId) []byte { return k[:] },
		func(bs []byte) *types.TokenId {
			var res types.TokenId
			copy(res[:], bs)
			return &res
		},
	}
}

func NewShardBlocksTrieReader(parent *mpt.Reader) *ShardBlocksTrieReader {
	return &ShardBlocksTrieReader{
		parent,
		func(k types.ShardId) []byte { return k.Bytes() },
		func(bs []byte) *types.ShardId { shard := types.BytesToShardId(bs); return &shard },
	}
}

func NewContractTrie(parent *mpt.MerklePatriciaTrie) *ContractTrie {
	return &ContractTrie{
		BaseMPTReader: NewContractTrieReader(parent.Reader),
		rwTrie:        parent,
	}
}

func NewTransactionTrie(parent *mpt.MerklePatriciaTrie) *TransactionTrie {
	return &TransactionTrie{
		BaseMPTReader: newTransactionTrieReader(parent.Reader),
		rwTrie:        parent,
	}
}

func NewTxCountTrie(parent *mpt.MerklePatriciaTrie) *TxCountTrie {
	// TODO: in order to make life easier for the proof system,
	// we will need to pad ShardId keys to the size of TransactionIndex.
	return &TxCountTrie{
		BaseMPTReader: newTxCountTrieReader(parent.Reader),
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
		BaseMPTReader: newAsyncContextTrieReader(parent.Reader),
		rwTrie:        parent,
	}
}

func NewTokenTrie(parent *mpt.MerklePatriciaTrie) *TokenTrie {
	return &TokenTrie{
		BaseMPTReader: NewTokenTrieReader(parent.Reader),
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

func NewDbTransactionTrieReader(tx db.RoTx, shardId types.ShardId) *TransactionTrieReader {
	return newTransactionTrieReader(mpt.NewDbReader(tx, shardId, db.TransactionTrieTable))
}

func NewDbTxCountTrieReader(tx db.RoTx, shardId types.ShardId) *TxCountTrieReader {
	return newTxCountTrieReader(mpt.NewDbReader(tx, shardId, db.TransactionTrieTable))
}

func NewDbReceiptTrieReader(tx db.RoTx, shardId types.ShardId) *ReceiptTrieReader {
	return NewReceiptTrieReader(mpt.NewDbReader(tx, shardId, db.ReceiptTrieTable))
}

func NewDbStorageTrieReader(tx db.RoTx, shardId types.ShardId) *StorageTrieReader {
	return NewStorageTrieReader(mpt.NewDbReader(tx, shardId, db.StorageTrieTable))
}

func NewDbAsyncContextTrieReader(tx db.RoTx, shardId types.ShardId) *AsyncContextTrieReader {
	return newAsyncContextTrieReader(mpt.NewDbReader(tx, shardId, db.AsyncCallContextTable))
}

func NewDbTokenTrieReader(tx db.RoTx, shardId types.ShardId) *TokenTrieReader {
	return NewTokenTrieReader(mpt.NewDbReader(tx, shardId, db.TokenTrieTable))
}

func NewDbShardBlocksTrieReader(
	tx db.RoTx,
	shardId types.ShardId,
	blockNumber types.BlockNumber,
) *ShardBlocksTrieReader {
	return NewShardBlocksTrieReader(mpt.NewDbReader(tx, shardId, db.ShardBlocksTrieTableName(blockNumber)))
}

func NewDbContractTrie(tx db.RwTx, shardId types.ShardId) *ContractTrie {
	return NewContractTrie(mpt.NewDbMPT(tx, shardId, db.ContractTrieTable))
}

func NewDbTransactionTrie(tx db.RwTx, shardId types.ShardId) *TransactionTrie {
	return NewTransactionTrie(mpt.NewDbMPT(tx, shardId, db.TransactionTrieTable))
}

func NewDbTxCountTrie(tx db.RwTx, shardId types.ShardId) *TxCountTrie {
	return NewTxCountTrie(mpt.NewDbMPT(tx, shardId, db.TransactionTrieTable))
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

func NewDbTokenTrie(tx db.RwTx, shardId types.ShardId) *TokenTrie {
	return NewTokenTrie(mpt.NewDbMPT(tx, shardId, db.TokenTrieTable))
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
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (m *BaseMPTReader[K, V, VPtr]) Items() iter.Seq2[K, V] {
	type Yield = func(K, V) bool
	return func(yield Yield) {
		for key, value := range m.Iterate() {
			k := m.keyFromBytes(key)
			if k == nil {
				continue
			}

			v := m.newV()
			if err := v.UnmarshalSSZ(value); err != nil {
				panic(err)
			}

			if !yield(*k, *v) {
				break
			}
		}
	}
}

func (m *BaseMPTReader[K, V, VPtr]) Entries() ([]Entry[K, VPtr], error) {
	res := make([]Entry[K, VPtr], 0)
	for key, value := range m.Iterate() {
		k := m.keyFromBytes(key)
		if k == nil {
			continue
		}

		v := m.newV()
		if err := v.UnmarshalSSZ(value); err != nil {
			return nil, err
		}

		res = append(res, Entry[K, VPtr]{*k, v})
	}
	return res, nil
}

func (m *BaseMPTReader[K, V, VPtr]) Keys() ([]K, error) {
	res := make([]K, 0)
	for key := range m.Iterate() {
		k := m.keyFromBytes(key)
		if k == nil {
			continue
		}
		res = append(res, *k)
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

func (m *BaseMPT[K, V, VPtr]) UpdateBatch(keys []K, values []VPtr) error {
	if len(keys) == 0 && len(values) == 0 {
		return nil
	}
	k := make([][]byte, 0, len(keys))
	v := make([][]byte, 0, len(values))
	for _, key := range keys {
		k = append(k, m.keyToBytes(key))
	}
	for _, value := range values {
		val, err := value.MarshalSSZ()
		if err != nil {
			return err
		}
		v = append(v, val)
	}
	return m.rwTrie.SetBatch(k, v)
}

func UpdateFromMap[K comparable, MV any, V any, VPtr MPTValue[V]](
	m *BaseMPT[K, V, VPtr],
	data map[K]MV,
	extract func(MV) VPtr,
) error {
	if len(data) == 0 {
		return nil
	}
	keys := make([][]byte, 0, len(data))
	values := make([][]byte, 0, len(data))
	for k, v := range data {
		keys = append(keys, m.keyToBytes(k))
		if extract != nil {
			val, err := extract(v).MarshalSSZ()
			if err != nil {
				return err
			}
			values = append(values, val)
		} else {
			v, ok := any(v).(VPtr)
			check.PanicIfNot(ok)

			val, err := v.MarshalSSZ()
			if err != nil {
				return err
			}
			values = append(values, val)
		}
	}
	return m.rwTrie.SetBatch(keys, values)
}

func (m *BaseMPT[K, V, VPtr]) Delete(key K) error {
	k := m.keyToBytes(key)
	return m.rwTrie.Delete(k)
}

func (m *BaseMPT[K, V, VPtr]) MPT() *mpt.MerklePatriciaTrie {
	return m.rwTrie
}

func ConvertTrieEntriesToMap[K comparable, V any](entries []Entry[K, *V]) map[K]V {
	return common.SliceToMap(
		entries,
		func(_ int, kv Entry[K, *V]) (K, V) {
			return kv.Key, *kv.Val
		})
}
