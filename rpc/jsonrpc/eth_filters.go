package jsonrpc

import (
	"context"
	"errors"
	"strings"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/rpc/filters"
	"github.com/holiman/uint256"
)

type LogsAggregator struct {
	filters *filters.FiltersManager
	logsMap *SyncMap[filters.SubscriptionID, []*types.Log]
}

func NewLogsAggregator(db db.DB) *LogsAggregator {
	return &LogsAggregator{
		filters: filters.NewFiltersManager(context.Background(), db, false),
		logsMap: NewSyncMap[filters.SubscriptionID, []*types.Log](), // make(map[filters.SubscriptionID][]*types.Log),
	}
}

func (l *LogsAggregator) CreateFilter(query *filters.FilterQuery) (filters.SubscriptionID, error) {
	id, filter := l.filters.NewFilter(query)
	if len(id) == 0 || filter == nil {
		return "", errors.New("cannot create new filter")
	}

	go func() {
		for log := range filter.LogsChannel() {
			l.logsMap.DoAndStore(id, func(st []*types.Log, ok bool) []*types.Log {
				if !ok {
					st = make([]*types.Log, 0)
				}
				st = append(st, log)
				return st
			})
		}
	}()

	return id, nil
}

func (l *LogsAggregator) GetLogs(id filters.SubscriptionID) ([]*types.Log, bool) {
	return l.logsMap.Delete(id)
}

// NewPendingTransactionFilter new transaction filter
func (api *APIImpl) NewPendingTransactionFilter(_ context.Context) (string, error) {
	return "", errNotImplemented
}

// NewBlockFilter implements eth_newBlockFilter. Creates a filter in the node, to notify when a new block arrives.
func (api *APIImpl) NewBlockFilter(_ context.Context) (string, error) {
	return "", errNotImplemented
}

// NewFilter implements eth_newFilter. Creates an arbitrary filter object, based on filter options, to notify when the state changes (logs).
func (api *APIImpl) NewFilter(_ context.Context, fromBlock *uint256.Int, toBlock *uint256.Int, address *common.Address, topics [][]common.Hash) (string, error) {
	query := filters.FilterQuery{FromBlock: fromBlock, ToBlock: toBlock, Address: address, Topics: topics}
	id, err := api.logs.CreateFilter(&query)
	if err != nil {
		return "", err
	}
	return "0x" + string(id), nil
}

// UninstallFilter new transaction filter
func (api *APIImpl) UninstallFilter(_ context.Context, id string) (isDeleted bool, err error) {
	id = strings.TrimPrefix(id, "0x")
	deleted := api.logs.filters.RemoveFilter(filters.SubscriptionID(id))
	return deleted, nil
}

// GetFilterChanges implements eth_getFilterChanges.
// Polling method for a previously-created filter
// returns an array of logs, block headers, or pending transactions which occurred since last poll.
func (api *APIImpl) GetFilterChanges(_ context.Context, id string) ([]any, error) {
	id = strings.TrimPrefix(id, "0x")
	logs, _ := api.logs.GetLogs(filters.SubscriptionID(id))
	res := make([]any, 0, len(logs))
	for _, log := range logs {
		res = append(res, log)
	}
	return res, nil
}

// GetFilterLogs implements eth_getFilterLogs.
// Polling method for a previously-created filter
// returns an array of logs which occurred since last poll.
func (api *APIImpl) GetFilterLogs(_ context.Context, id string) ([]*types.Log, error) {
	// TODO: It is legacy from Erigon, probably we need to fix it. The problem: seems that we need to return all logs
	// matching the criteria, but we return only changes since last Poll.
	id = strings.TrimPrefix(id, "0x")
	logs, _ := api.logs.GetLogs(filters.SubscriptionID(id))
	return logs, nil
}
