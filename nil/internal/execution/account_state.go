package execution

import (
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/mpt"
	"github.com/NilFoundation/nil/nil/internal/tracing"
	"github.com/NilFoundation/nil/nil/internal/types"
)

type JournalAppender interface {
	AppendToJournal(entry JournalEntry)
}

type DbRwTxProvider interface {
	GetRwTx() db.RwTx
}

type AccountWriter interface {
	SetAddress(*types.Address)

	AddBalance(amount types.Value, reason tracing.BalanceChangeReason) error
	SubBalance(amount types.Value, reason tracing.BalanceChangeReason) error
	SetBalance(amount types.Value)

	SetState(key common.Hash, value common.Hash) error
	SetStorage(storage Storage)

	SetTokenBalance(id types.TokenId, amount types.Value)

	SetSeqno(seqno types.Seqno)
	SetExtSeqno(seqno types.Seqno)

	SetCode(codeHash common.Hash, code []byte)

	SetAsyncContext(index types.TransactionIndex, ctx *types.AsyncContext)
	UndoSetAsyncContext(index types.TransactionIndex)
	GetAndRemoveAsyncContext(index types.TransactionIndex) (*types.AsyncContext, error)

	SetIsNew(val bool)
	SetIsSelfDestructed(val bool)

	Commit() (*types.SmartContract, error)
}

type AccountReader interface {
	GetAddress() *types.Address

	GetBalance() types.Value
	GetState(key common.Hash) (common.Hash, error)
	GetCommittedState(key common.Hash) (common.Hash, error)
	GetFullState() Storage
	GetStorageRoot() common.Hash

	GetTokenBalance(id types.TokenId) *types.Value
	GetTokens() map[types.TokenId]types.Value

	GetSeqno() types.Seqno
	GetExtSeqno() types.Seqno

	GetCode() types.Code
	GetCodeHash() common.Hash

	GetAsyncContext(index types.TransactionIndex) (*types.AsyncContext, error)
	GetAllAsyncContexts() map[types.TransactionIndex]*types.AsyncContext

	IsNew() bool
	IsSelfDestructed() bool
	IsEmpty() bool
}

type AccountState interface {
	AccountReader
	AccountWriter
}

type AccountStateImpl struct {
	dbRwTxProvider DbRwTxProvider
	address        types.Address // address of the ethereum account

	Balance     types.Value
	Code        types.Code
	CodeHash    common.Hash
	Seqno       types.Seqno
	ExtSeqno    types.Seqno
	StorageTree *StorageTrie
	TokenTree   *TokenTrie
	// AsyncContextTree is a trie that stores the context for each request sent from this account.
	AsyncContextTree *AsyncContextTrie

	State               Storage
	AsyncContext        map[types.TransactionIndex]*types.AsyncContext
	AsyncContextRemoved []types.TransactionIndex
	// Tokens holds the token changed during execution. If execution fails, these changes will be dropped.
	Tokens map[types.TokenId]*types.Value

	// Flag whether the account was marked as self-destructed. The self-destructed
	// account is still accessible in the scope of same transaction.
	selfDestructed bool

	// This is an EIP-6780 flag indicating whether the object is eligible for
	// self-destruct, according to EIP-6780. The flag could be set either when
	// the contract is just created within the current transaction, or when the
	// object was previously existent and is being deployed as a contract within
	// the current transaction.
	newContract bool

	logger logging.Logger
}

var _ AccountState = (*AccountStateImpl)(nil)

func NewAccountState(
	dbRwTxProvider DbRwTxProvider,
	addr types.Address,
	account *types.SmartContract,
	logger logging.Logger,
) (AccountState, error) {
	shardId := addr.ShardId()

	rwTx := dbRwTxProvider.GetRwTx()
	accountState := &AccountStateImpl{
		dbRwTxProvider:   dbRwTxProvider,
		address:          addr,
		TokenTree:        NewDbTokenTrie(rwTx, shardId),
		StorageTree:      NewDbStorageTrie(rwTx, shardId),
		AsyncContextTree: NewDbAsyncContextTrie(rwTx, shardId),
		CodeHash:         types.EmptyCodeHash,

		State:        make(Storage),
		AsyncContext: make(map[types.TransactionIndex]*types.AsyncContext),
		Tokens:       make(map[types.TokenId]*types.Value),
		logger:       logger,
	}

	if account != nil {
		accountState.Balance = account.Balance
		accountState.TokenTree.SetRootHash(account.TokenRoot)
		accountState.StorageTree.SetRootHash(account.StorageRoot)
		accountState.CodeHash = account.CodeHash
		accountState.AsyncContextTree.SetRootHash(account.AsyncContextRoot)
		var err error
		accountState.Code, err = db.ReadCode(rwTx, shardId, account.CodeHash)
		if err != nil {
			return nil, fmt.Errorf("failed to read contract code: %w", err)
		}
		accountState.ExtSeqno = account.ExtSeqno
		accountState.Seqno = account.Seqno
	}

	return accountState, nil
}

func (asr *AccountStateImpl) GetAddress() *types.Address {
	return &asr.address
}

func (asr *AccountStateImpl) SetAddress(addr *types.Address) {
	asr.address = *addr
}

func (as *AccountStateImpl) GetBalance() types.Value {
	return as.Balance
}

// AddBalance adds amount to as's balance.
// It is used to add funds to the destination account of a transfer.
func (as *AccountStateImpl) AddBalance(amount types.Value, reason tracing.BalanceChangeReason) error {
	if amount.IsZero() {
		return nil
	}
	newBalance, overflow := as.Balance.AddOverflow(amount)
	if overflow {
		return fmt.Errorf("balance overflow: %s + %s", as.Balance, amount)
	}

	as.logger.Debug().Stringer("address", as.address).Stringer("reason", reason).
		Msgf("Balance change: adding balance %s + %s = %s", as.Balance, amount, newBalance)
	as.SetBalance(newBalance)
	return nil
}

// SubBalance removes amount from as's balance.
// It is used to remove funds from the origin account of a transfer.
func (as *AccountStateImpl) SubBalance(amount types.Value, reason tracing.BalanceChangeReason) error {
	if amount.IsZero() {
		return nil
	}
	newBalance, overflow := as.Balance.SubOverflow(amount)
	if overflow {
		return fmt.Errorf("balance overflow: %s + %s", as.Balance, amount)
	}

	as.logger.Debug().Stringer("address", as.address).Stringer("reason", reason).
		Msgf("Balance change: withdrawing balance %s - %s = %s", as.Balance, amount, newBalance)
	as.SetBalance(newBalance)
	return nil
}

func (as *AccountStateImpl) SetBalance(amount types.Value) {
	as.Balance = amount
}

func (as *AccountStateImpl) GetState(key common.Hash) (common.Hash, error) {
	val, ok := as.State[key]
	if ok {
		return val, nil
	}

	newVal, err := as.GetCommittedState(key)
	if err != nil {
		return common.EmptyHash, err
	}
	as.State[key] = newVal
	return newVal, nil
}

// GetCommittedState retrieves a value from the committed account storage trie.
func (as *AccountStateImpl) GetCommittedState(key common.Hash) (common.Hash, error) {
	res, err := as.StorageTree.Fetch(key)
	if errors.Is(err, db.ErrKeyNotFound) {
		return common.EmptyHash, nil
	}
	if err != nil {
		return common.EmptyHash, err
	}

	return res.Bytes32(), nil
}

// GetFullState retrieves full state of the account as a map.
func (as *AccountStateImpl) GetFullState() Storage {
	return as.State
}

func (asr *AccountStateImpl) SetState(key common.Hash, value common.Hash) error {
	asr.State[key] = value
	return nil
}

// SetStorage replaces the entire state storage with the given one.
//
// After this function is called, all original state will be ignored and state
// lookup only happens in the fake state storage.
//
// Note this function should only be used for debugging purpose.
func (as *AccountStateImpl) SetStorage(storage Storage) {
	for key, value := range storage {
		as.State[key] = value
	}
	// Don't bother journal since this function should only be used for
	// debugging and the `fake` storage won't be committed to database.
}

// GetStorageRoot retrieves the root hash of the storage.
func (as *AccountStateImpl) GetStorageRoot() common.Hash {
	storageRootHash := as.StorageTree.RootHash()
	if storageRootHash == mpt.EmptyRootHash {
		return common.EmptyHash
	}
	return storageRootHash
}

func (as *AccountStateImpl) GetTokenBalance(id types.TokenId) *types.Value {
	if value, exists := as.Tokens[id]; exists {
		return value
	}

	tokenBalance, err := as.TokenTree.Fetch(id)
	if errors.Is(err, db.ErrKeyNotFound) {
		return nil
	}
	check.PanicIfErr(err)

	as.Tokens[id] = tokenBalance
	return tokenBalance
}

func (as *AccountStateImpl) SetTokenBalance(id types.TokenId, amount types.Value) {
	as.Tokens[id] = &amount
	as.logger.Debug().
		Stringer("address", as.address).
		Hex("id", id[:]).
		Stringer("amount", amount).
		Msg("Set balance token")
}

func (as *AccountStateImpl) GetTokens() map[types.TokenId]types.Value {
	res := make(map[types.TokenId]types.Value)
	for k, v := range as.TokenTree.Iterate() {
		var c types.TokenBalance
		c.Token = types.TokenId(k)
		if err := c.Balance.UnmarshalNil(v); err != nil {
			as.logger.Error().Err(err).Msg("failed to unmarshal token balance")
			continue
		}
		res[c.Token] = c.Balance
	}
	// If some token was changed during execution, we need to set it to the result. It will probably rewrite values
	// fetched from the storage above.
	for id, balance := range as.Tokens {
		if balance != nil {
			res[id] = *balance
		}
	}
	return res
}

func (as *AccountStateImpl) GetSeqno() types.Seqno {
	return as.Seqno
}

func (as *AccountStateImpl) SetSeqno(seqno types.Seqno) {
	as.Seqno = seqno
}

func (as *AccountStateImpl) GetExtSeqno() types.Seqno {
	return as.ExtSeqno
}

func (as *AccountStateImpl) SetExtSeqno(seqno types.Seqno) {
	as.ExtSeqno = seqno
}

func (as *AccountStateImpl) GetCode() types.Code {
	return as.Code
}

func (as *AccountStateImpl) SetCode(codeHash common.Hash, code []byte) {
	as.Code = code
	as.CodeHash = common.Hash(codeHash[:])
}

func (as *AccountStateImpl) GetCodeHash() common.Hash {
	return as.CodeHash
}

func (as *AccountStateImpl) GetAsyncContext(index types.TransactionIndex) (*types.AsyncContext, error) {
	ctx, exists := as.AsyncContext[index]
	if exists {
		return ctx, nil
	}
	return as.AsyncContextTree.Fetch(index)
}

func (as *AccountStateImpl) GetAllAsyncContexts() map[types.TransactionIndex]*types.AsyncContext {
	return as.AsyncContext
}

func (as *AccountStateImpl) SetAsyncContext(index types.TransactionIndex, ctx *types.AsyncContext) {
	as.AsyncContext[index] = ctx
}

func (as *AccountStateImpl) UndoSetAsyncContext(index types.TransactionIndex) {
	delete(as.AsyncContext, index)
}

func (as *AccountStateImpl) GetAndRemoveAsyncContext(index types.TransactionIndex) (*types.AsyncContext, error) {
	_, exists := as.AsyncContext[index]
	check.PanicIff(exists, "AsyncContext %d already exists", index)
	as.AsyncContextRemoved = append(as.AsyncContextRemoved, index)
	return as.AsyncContextTree.Fetch(index)
}

func (as *AccountStateImpl) Commit() (*types.SmartContract, error) {
	// Remove zero values from the state cache and the storage trie
	for key, value := range as.State {
		if value == common.EmptyHash {
			delete(as.State, key)
			if err := as.StorageTree.Delete(key); err != nil && !errors.Is(err, db.ErrKeyNotFound) {
				return nil, fmt.Errorf("failed to delete key %s: %w", key, err)
			}
		}
	}
	// Update storage trie with the new values
	if err := UpdateFromMap(
		as.StorageTree,
		as.State,
		func(v common.Hash) *types.Uint256 {
			return (*types.Uint256)(v.Uint256())
		}); err != nil {
		return nil, err
	}

	if err := UpdateFromMap(as.AsyncContextTree, as.AsyncContext, nil); err != nil {
		return nil, err
	}

	for _, k := range as.AsyncContextRemoved {
		if err := as.AsyncContextTree.Delete(k); err != nil {
			return nil, err
		}
	}

	// Remove tokens with zero value
	for k, v := range as.Tokens {
		if v.IsZero() {
			// We ignore `db.ErrKeyNotFound` error because there is a possibility that the token was created during
			// execution of the current transaction, and it is not in the trie.
			if err := as.TokenTree.Delete(k); err != nil && !errors.Is(err, db.ErrKeyNotFound) {
				return nil, err
			}
			delete(as.Tokens, k)
		}
	}

	if err := UpdateFromMap(as.TokenTree, as.Tokens, nil); err != nil {
		return nil, err
	}

	acc := &types.SmartContract{
		Address:          as.address,
		Balance:          as.Balance,
		StorageRoot:      as.StorageTree.RootHash(),
		TokenRoot:        as.TokenTree.RootHash(),
		AsyncContextRoot: as.AsyncContextTree.RootHash(),
		CodeHash:         as.CodeHash,
		ExtSeqno:         as.ExtSeqno,
		Seqno:            as.Seqno,
	}

	// TODO: dbRwTxProvider is used only here, move WriteCode func to inteface
	if err := db.WriteCode(as.dbRwTxProvider.GetRwTx(), as.address.ShardId(), as.CodeHash, as.Code); err != nil {
		return nil, err
	}

	return acc, nil
}

// IsNew returns if the account has been just created.
func (as *AccountStateImpl) IsNew() bool {
	return as.newContract
}

// SetIsNew marks account as just created
func (as *AccountStateImpl) SetIsNew(val bool) {
	as.newContract = val
}

// IsSelfDestructed returns if the account has been marked as self-destructed.
func (as *AccountStateImpl) IsSelfDestructed() bool {
	return as.selfDestructed
}

// SetSelfDestructed marks account as self-destructed
func (as *AccountStateImpl) SetIsSelfDestructed(val bool) {
	as.selfDestructed = val
}

// IsEmpty returns if the account is considered empty according to EIP-161.
func (asr *AccountStateImpl) IsEmpty() bool {
	return asr.Seqno == 0 && asr.Balance.IsZero() && len(asr.Code) == 0
}

type JournaledAccountState struct {
	AccountState
	journalAppender JournalAppender
	logger          logging.Logger
}

var _ AccountState = (*JournaledAccountState)(nil)

// NewJournaledAccountStateWrapper takes AccountState as input and adds journalling to it
func NewJournaledAccountStateFromRaw(
	journalAppender JournalAppender,
	accountState AccountState,
	logger logging.Logger,
) AccountState {
	return &JournaledAccountState{
		AccountState:    accountState,
		journalAppender: journalAppender,
		logger:          logger,
	}
}

// NewJournaledAccountState creates raw account, then wraps it into journalled account.
// func NewJournaledAccountState(
// 	dbRwTxProvider DbRwTxProvider,
// 	journalAppender JournalAppender,
// 	addr types.Address,
// 	account *types.SmartContract,
// 	logger logging.Logger,
// ) (AccountState, error) {
// 	acccountStateRaw, err := NewAccountState(
// 		dbRwTxProvider,
// 		addr,
// 		account,
// 		logger,
// 	)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return NewJournaledAccountStateFromRaw(journalAppender, acccountStateRaw, logger), nil
// }

func (as *JournaledAccountState) SetBalance(amount types.Value) {
	as.journalAppender.AppendToJournal(balanceChange{
		account: as.GetAddress(),
		prev:    as.GetBalance(),
	})
	as.AccountState.SetBalance(amount)
}

func (jas *JournaledAccountState) SetState(key common.Hash, value common.Hash) error {
	// If the new value is the same as old, don't set. Otherwise, track only the
	// dirty changes, supporting reverting all of it back to no change.
	prev, err := jas.GetState(key)
	if err != nil {
		return err
	}
	if prev == value {
		return nil
	}
	// New value is different, update and journal the change
	jas.journalAppender.AppendToJournal(storageChange{
		account:   jas.GetAddress(),
		key:       key,
		prevvalue: prev,
	})
	return jas.AccountState.SetState(key, value)
}

func (as *JournaledAccountState) SetTokenBalance(id types.TokenId, amount types.Value) {
	prev := as.GetTokenBalance(id)
	change := tokenChange{
		account: as.GetAddress(),
		id:      id,
	}
	if prev != nil {
		change.prev = *prev
	}
	as.journalAppender.AppendToJournal(change)
	as.AccountState.SetTokenBalance(id, amount)
}

func (as *JournaledAccountState) SetSeqno(seqno types.Seqno) {
	as.journalAppender.AppendToJournal(seqnoChange{
		account: as.GetAddress(),
		prev:    as.GetSeqno(),
	})
	as.AccountState.SetSeqno(seqno)
}

func (as *JournaledAccountState) SetExtSeqno(seqno types.Seqno) {
	as.journalAppender.AppendToJournal(extSeqnoChange{
		account: as.GetAddress(),
		prev:    as.GetExtSeqno(),
	})
	as.AccountState.SetExtSeqno(seqno)
}

func (as *JournaledAccountState) SetCode(codeHash common.Hash, code []byte) {
	as.journalAppender.AppendToJournal(codeChange{
		account:  as.GetAddress(),
		prevhash: as.GetCodeHash().Bytes(),
		prevcode: as.GetCode(),
	})
	as.AccountState.SetCode(codeHash, code)
}

func (as *JournaledAccountState) SetAsyncContext(index types.TransactionIndex, ctx *types.AsyncContext) {
	as.journalAppender.AppendToJournal(asyncContextChange{
		account:   as.GetAddress(),
		requestId: index,
	})
	as.AccountState.SetAsyncContext(index, ctx)
}
