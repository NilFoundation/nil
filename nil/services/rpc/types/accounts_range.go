package types

// type RangedAccount struct {
// 	Balance     types.Value                 `json:"balance"`
// 	Nonce       types.Seqno                 `json:"nonce"`
// 	StorageRoot common.Hash                 `json:"root"`
// 	CodeHash    common.Hash                 `json:"codeHash"`
// 	Code        hexutil.Bytes               `json:"code,omitempty"`
// 	Storage     map[common.Hash]hexutil.Big `json:"storage,omitempty"`
// 	Address     types.Address               `json:"address"`
// 	AddressHash common.Hash                 `json:"key"`
// }

// type AccountsRange struct {
// 	Root     common.Hash                      `json:"root"`
// 	Accounts map[types.Address]*RangedAccount `json:"accounts"`
// 	// `Next` can be set to represent that this range is only partial, and `Next`
// 	// is where an iterator should be positioned in order to continue the range.
// 	Next *common.Hash `json:"next,omitempty"` // nil if no more accounts
// }
