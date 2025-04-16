package types

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
)

// @component Contract contract object "Overriding fields of contract during the execution of a transaction call."
// @componentprop Seqno seqno integer true "The sequence number of the smart contract."
// @componentprop ExtSeqno extSeqno integer true "The external sequence number of the smart contract."
// @componentprop Code code string true "Smart contract code."
// @componentprop Balance balance integer true "Account balance."
// @componentprop State state map true "Key-value pairs should be used as an account state."
// @componentprop StateDiff stateDiff map true "Key-value pairs should be applied to account state."

type Contract struct {
	Seqno        *types.Seqno                                    `json:"seqno"`
	ExtSeqno     *types.Seqno                                    `json:"extSeqno"`
	Code         *hexutil.Bytes                                  `json:"code"`
	Balance      *types.Value                                    `json:"balance"`
	State        *map[common.Hash]common.Hash                    `json:"state"`
	StateDiff    *map[common.Hash]common.Hash                    `json:"stateDiff"`
	AsyncContext *map[types.TransactionIndex]*types.AsyncContext `json:"asyncContext"`
}

type StateOverrides map[types.Address]Contract

func (overrides *StateOverrides) Override(state *execution.ExecutionState) error {
	for addr, account := range *overrides {
		if addr.ShardId() != state.ShardId {
			continue
		}

		// Override contract seqno.
		if account.Seqno != nil {
			if err := state.SetSeqno(addr, *account.Seqno); err != nil {
				return err
			}
		}
		// Override contract external seqno.
		if account.ExtSeqno != nil {
			if err := state.SetExtSeqno(addr, *account.ExtSeqno); err != nil {
				return err
			}
		}
		// Override contract code.
		if account.Code != nil {
			if err := state.SetCode(addr, *account.Code); err != nil {
				return err
			}
		}
		// Override account balance.
		if account.Balance != nil {
			if err := state.SetBalance(addr, *account.Balance); err != nil {
				return err
			}
		}
		if account.State != nil && account.StateDiff != nil {
			return fmt.Errorf("account %s has both 'state' and 'stateDiff'", addr.Hex())
		}
		// Replace entire state if caller requires.
		if account.State != nil {
			if err := state.SetStorage(addr, *account.State); err != nil {
				return err
			}
		}
		// Apply state diff for a specified contract.
		if account.StateDiff != nil {
			for key, value := range *account.StateDiff {
				if err := state.SetState(addr, key, value); err != nil {
					return err
				}
			}
		}
		// Apply async context for a specified contract.
		if account.AsyncContext != nil {
			for key, value := range *account.AsyncContext {
				if err := state.SetAsyncContext(addr, key, value); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
