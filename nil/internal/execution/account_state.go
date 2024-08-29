package execution

import (
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/tracing"
	"github.com/NilFoundation/nil/nil/internal/types"
)

type AccountStateReader struct {
	CurrencyTrieReader *CurrencyTrieReader
}

func (asr *AccountStateReader) GetCurrencyBalance(id types.CurrencyId) types.Value {
	res, err := asr.CurrencyTrieReader.Fetch(id)
	if errors.Is(err, db.ErrKeyNotFound) {
		return types.Value{}
	}
	check.PanicIfErr(err)
	return *res
}

type AccountState struct {
	db      *ExecutionState
	address types.Address // address of the ethereum account

	Tx           db.RwTx
	Balance      types.Value
	Code         types.Code
	CodeHash     common.Hash
	Seqno        types.Seqno
	ExtSeqno     types.Seqno
	StorageTree  *StorageTrie
	CurrencyTree *CurrencyTrie
	// AsyncContextTree is a trie that stores the context for each request sent from this account.
	AsyncContextTree *AsyncContextTrie
	// requestId is a current request id. It is used to generate unique number for each request.
	requestId uint64

	State               Storage
	AsyncContext        map[types.MessageIndex]*types.AsyncContext
	AsyncContextRemoved []types.MessageIndex

	// Flag whether the account was marked as self-destructed. The self-destructed
	// account is still accessible in the scope of same transaction.
	selfDestructed bool

	// This is an EIP-6780 flag indicating whether the object is eligible for
	// self-destruct, according to EIP-6780. The flag could be set either when
	// the contract is just created within the current transaction, or when the
	// object was previously existent and is being deployed as a contract within
	// the current transaction.
	newContract bool
}

// FetchRequestId returns unique request id.
func (as *AccountState) FetchRequestId() uint64 {
	as.requestId += 1
	return as.requestId
}

func NewAccountStateReader(account *AccountState) *AccountStateReader {
	return &AccountStateReader{
		CurrencyTrieReader: account.CurrencyTree.BaseMPTReader,
	}
}

func NewAccountState(es *ExecutionState, addr types.Address, account *types.SmartContract) (*AccountState, error) {
	shardId := addr.ShardId()

	accountState := &AccountState{
		db:               es,
		address:          addr,
		CurrencyTree:     NewDbCurrencyTrie(es.tx, shardId),
		StorageTree:      NewDbStorageTrie(es.tx, shardId),
		AsyncContextTree: NewDbAsyncContextTrie(es.tx, shardId),

		Tx:           es.tx,
		State:        make(Storage),
		AsyncContext: make(map[types.MessageIndex]*types.AsyncContext),
	}

	if account != nil {
		accountState.Balance = account.Balance
		accountState.CurrencyTree.SetRootHash(account.CurrencyRoot)
		accountState.StorageTree.SetRootHash(account.StorageRoot)
		accountState.CodeHash = account.CodeHash
		accountState.AsyncContextTree.SetRootHash(account.AsyncContextRoot)
		var err error
		accountState.Code, err = db.ReadCode(es.tx, shardId, account.CodeHash)
		if err != nil {
			return nil, err
		}
		accountState.ExtSeqno = account.ExtSeqno
		accountState.Seqno = account.Seqno
		accountState.requestId = account.RequestId
	}

	return accountState, nil
}

func (as *AccountState) empty() bool {
	return as.Seqno == 0 && as.Balance.IsZero() && len(as.Code) == 0
}

// AddBalance adds amount to s's balance.
// It is used to add funds to the destination account of a transfer.
func (as *AccountState) AddBalance(amount types.Value, reason tracing.BalanceChangeReason) error {
	if amount.IsZero() {
		return nil
	}
	newBalance, overflow := as.Balance.AddOverflow(amount)
	if overflow {
		return fmt.Errorf("balance overflow: %s + %s", as.Balance, amount)
	}

	logger.Debug().Stringer("address", as.address).Stringer("reason", reason).
		Msgf("Balance change: adding balance %s + %s = %s", as.Balance, amount, newBalance)
	as.SetBalance(newBalance)
	return nil
}

// SubBalance removes amount from s's balance.
// It is used to remove funds from the origin account of a transfer.
func (as *AccountState) SubBalance(amount types.Value, reason tracing.BalanceChangeReason) error {
	if amount.IsZero() {
		return nil
	}
	newBalance, overflow := as.Balance.SubOverflow(amount)
	if overflow {
		return fmt.Errorf("balance overflow: %s + %s", as.Balance, amount)
	}

	logger.Debug().Stringer("address", as.address).Stringer("reason", reason).
		Msgf("Balance change: withdrawing balance %s - %s = %s", as.Balance, amount, newBalance)
	as.SetBalance(newBalance)
	return nil
}

func (as *AccountState) GetState(key common.Hash) (common.Hash, error) {
	val, ok := as.State[key]
	if ok {
		return val, nil
	}

	newVal, err := as.GetCommittedState(key)
	if err != nil {
		return common.Hash{}, err
	}
	as.State[key] = newVal
	return newVal, nil
}

func (as *AccountState) SetBalance(amount types.Value) {
	as.db.journal.append(balanceChange{
		account: &as.address,
		prev:    as.Balance,
	})
	as.setBalance(amount)
}

func (as *AccountState) setBalance(amount types.Value) {
	as.Balance = amount
}

func (as *AccountState) SetCurrencyBalance(id types.CurrencyId, amount types.Value) {
	prev, err := as.CurrencyTree.Fetch(id)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		panic(err)
	}
	change := currencyChange{
		account: &as.address,
		id:      id,
	}
	if prev != nil {
		change.prev = *prev
	}
	as.db.journal.append(change)
	as.setCurrencyBalance(id, amount)
}

func (as *AccountState) setCurrencyBalance(id types.CurrencyId, amount types.Value) {
	check.PanicIfErr(as.CurrencyTree.Update(id, &amount))
	logger.Debug().
		Stringer("address", as.address).
		Hex("id", id[:]).
		Stringer("amount", amount).
		Msg("Set balance currency")
}

func (as *AccountState) GetCurrencyBalance(id types.CurrencyId) types.Value {
	res, err := as.CurrencyTree.Fetch(id)
	if errors.Is(err, db.ErrKeyNotFound) {
		return types.Value{}
	}
	check.PanicIfErr(err)
	return *res
}

func (as *AccountState) SetSeqno(seqno types.Seqno) {
	as.db.journal.append(seqnoChange{
		account: &as.address,
		prev:    as.Seqno,
	})
	as.Seqno = seqno
}

func (as *AccountState) SetExtSeqno(seqno types.Seqno) {
	as.db.journal.append(extSeqnoChange{
		account: &as.address,
		prev:    as.ExtSeqno,
	})
	as.ExtSeqno = seqno
}

func (as *AccountState) SetCode(codeHash common.Hash, code []byte) {
	prevcode := as.Code
	as.db.journal.append(codeChange{
		account:  &as.address,
		prevhash: as.CodeHash[:],
		prevcode: prevcode,
	})
	as.setCode(codeHash, code)
}

func (as *AccountState) SetAsyncContext(index types.MessageIndex, ctx *types.AsyncContext) {
	as.db.journal.append(asyncContextChange{
		account:   &as.address,
		requestId: index,
	})
	as.AsyncContext[index] = ctx
}

func (as *AccountState) GetAndRemoveAsyncContext(index types.MessageIndex) (*types.AsyncContext, error) {
	ctx, exists := as.AsyncContext[index]
	if exists {
		return ctx, nil
	}
	as.AsyncContextRemoved = append(as.AsyncContextRemoved, index)
	return as.AsyncContextTree.Fetch(index)
}

func (as *AccountState) setCode(codeHash common.Hash, code []byte) {
	as.Code = code
	as.CodeHash = common.Hash(codeHash[:])
}

func (as *AccountState) SetState(key common.Hash, value common.Hash) error {
	// If the new value is the same as old, don't set. Otherwise, track only the
	// dirty changes, supporting reverting all of it back to no change.
	prev, err := as.GetState(key)
	if err != nil {
		return err
	}
	if prev == value {
		return nil
	}
	// New value is different, update and journal the change
	as.db.journal.append(storageChange{
		account:   &as.address,
		key:       key,
		prevvalue: prev,
	})
	as.setState(key, value)
	return nil
}

// SetStorage replaces the entire state storage with the given one.
//
// After this function is called, all original state will be ignored and state
// lookup only happens in the fake state storage.
//
// Note this function should only be used for debugging purpose.
func (as *AccountState) SetStorage(storage Storage) {
	for key, value := range storage {
		as.State[key] = value
	}
	// Don't bother journal since this function should only be used for
	// debugging and the `fake` storage won't be committed to database.
}

func (as *AccountState) setState(key common.Hash, value common.Hash) {
	as.State[key] = value
}

// GetCommittedState retrieves a value from the committed account storage trie.
func (as *AccountState) GetCommittedState(key common.Hash) (common.Hash, error) {
	res, err := as.StorageTree.Fetch(key)
	if errors.Is(err, db.ErrKeyNotFound) {
		return common.EmptyHash, nil
	}
	if err != nil {
		return common.EmptyHash, err
	}

	return res.Bytes32(), nil
}

func (as *AccountState) Commit() (*types.SmartContract, error) {
	for k, v := range as.State {
		if err := as.StorageTree.Update(k, (*types.Uint256)(v.Uint256())); err != nil {
			return nil, err
		}
	}

	for k, v := range as.AsyncContext {
		if err := as.AsyncContextTree.Update(k, v); err != nil {
			return nil, err
		}
	}

	for _, k := range as.AsyncContextRemoved {
		if err := as.AsyncContextTree.Delete(k); err != nil {
			return nil, err
		}
	}

	acc := &types.SmartContract{
		Address:          as.address,
		Balance:          as.Balance,
		StorageRoot:      as.StorageTree.RootHash(),
		CurrencyRoot:     as.CurrencyTree.RootHash(),
		AsyncContextRoot: as.AsyncContextTree.RootHash(),
		CodeHash:         as.CodeHash,
		ExtSeqno:         as.ExtSeqno,
		Seqno:            as.Seqno,
		RequestId:        as.requestId,
	}

	if err := db.WriteCode(as.Tx, as.address.ShardId(), as.Code); err != nil {
		return nil, err
	}

	return acc, nil
}
