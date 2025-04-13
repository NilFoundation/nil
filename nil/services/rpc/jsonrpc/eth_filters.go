package jsonrpc

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/NilFoundation/nil/nil/common/concurrent"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/filters"
)

type LogsAggregator struct {
	filters   *filters.FiltersManager
	logsMap   *concurrent.Map[filters.SubscriptionID, []*filters.MetaLog]
	blocksMap *concurrent.Map[filters.SubscriptionID, []*types.Block]
}

func NewLogsAggregator(ctx context.Context, db db.ReadOnlyDB, pollBlocksForLogs bool) *LogsAggregator {
	return &LogsAggregator{
		filters:   filters.NewFiltersManager(ctx, db, !pollBlocksForLogs),
		logsMap:   concurrent.NewMap[filters.SubscriptionID, []*filters.MetaLog](),
		blocksMap: concurrent.NewMap[filters.SubscriptionID, []*types.Block](),
	}
}

func (l *LogsAggregator) WaitForShutdown() {
	l.filters.WaitForShutdown()
}

func (l *LogsAggregator) CreateFilter(query *filters.FilterQuery) (filters.SubscriptionID, error) {
	id, filter := l.filters.NewFilter(query)
	if len(id) == 0 || filter == nil {
		return "", errors.New("cannot create new filter")
	}

	go func() {
		for log := range filter.LogsChannel() {
			l.logsMap.DoAndStore(id, func(st []*filters.MetaLog, ok bool) []*filters.MetaLog {
				return append(st, log)
			})
		}
	}()

	return id, nil
}

func (l *LogsAggregator) CreateBlocksListener() (filters.SubscriptionID, error) {
	id, ch := l.filters.AddBlocksListener()
	if ch == nil {
		return "", errors.New("cannot add blocks listener")
	}

	go func() {
		for block := range ch {
			l.blocksMap.DoAndStore(id, func(t []*types.Block, ok bool) []*types.Block {
				return append(t, block)
			})
		}
	}()

	l.blocksMap.Put(id, []*types.Block{})
	return id, nil
}

func (l *LogsAggregator) RemoveBlocksListener(id filters.SubscriptionID) error {
	removed := l.filters.RemoveBlocksListener(id)
	if removed {
		return nil
	}
	return errors.New("cannot remove blocks listener")
}

func (l *LogsAggregator) GetLogs(id filters.SubscriptionID) ([]*filters.MetaLog, bool) {
	return l.logsMap.Delete(id)
}

// NewPendingTransactionFilter implements eth_newPendingTransactionFilter. It creates new transaction filter.
func (api *APIImplRo) NewPendingTransactionFilter(_ context.Context) (string, error) {
	return "", errNotImplemented
}

// NewBlockFilter implements eth_newBlockFilter. Creates a filter in the node, to notify when a new block arrives.
func (api *APIImplRo) NewBlockFilter(_ context.Context) (string, error) {
	id, err := api.logs.CreateBlocksListener()
	return string(id), err
}

// NewFilter implements eth_newFilter.
// Creates an arbitrary filter object, based on filter options, to notify when the state changes (logs).
func (api *APIImplRo) NewFilter(_ context.Context, query filters.FilterQuery) (string, error) {
	id, err := api.logs.CreateFilter(&query)
	if err != nil {
		return "", err
	}
	api.logger.Debug().Msgf("New filter created with id: %s", id)
	return "0x" + string(id), nil
}

// UninstallFilter implements eth_uninstallFilter.
func (api *APIImplRo) UninstallFilter(_ context.Context, id string) (isDeleted bool, err error) {
	id = strings.TrimPrefix(id, "0x")
	deleted := false
	if ok := api.logs.filters.RemoveFilter(filters.SubscriptionID(id)); ok {
		deleted = true
	}
	if err := api.logs.RemoveBlocksListener(filters.SubscriptionID(id)); err == nil {
		api.logs.blocksMap.Delete(filters.SubscriptionID(id))
		deleted = true
	}
	return deleted, nil
}

// GetFilterChanges implements eth_getFilterChanges.
// Polling method for a previously-created filter
// returns an array of logs, block headers, or pending transactions which occurred since last poll.
func (api *APIImplRo) GetFilterChanges(_ context.Context, id string) ([]any, error) {
	id = strings.TrimPrefix(id, "0x")
	if logs, ok := api.logs.GetLogs(filters.SubscriptionID(id)); ok {
		res := make([]any, 0, len(logs))
		for _, log := range logs {
			res = append(res, NewRPCLog(log.Log, log.BlockId))
		}
		return res, nil
	}
	res := make([]any, 0)
	_, ok := api.logs.blocksMap.DoAndStore(filters.SubscriptionID(id),
		func(blocks []*types.Block, ok bool) []*types.Block {
			for _, block := range blocks {
				res = append(res, block)
			}
			return []*types.Block{}
		})
	if ok {
		return res, nil
	}

	return nil, fmt.Errorf("filter does not exist: %s", id)
}

// GetFilterLogs implements eth_getFilterLogs.
// Polling method for a previously-created filter
// returns an array of logs which occurred since last poll.
func (api *APIImplRo) GetFilterLogs(_ context.Context, id string) ([]*RPCLog, error) {
	// TODO: It is legacy from Erigon, probably we need to fix it. The problem: seems that we need to return all logs
	// matching the criteria, but we return only changes since last Poll.
	id = strings.TrimPrefix(id, "0x")
	logs, ok := api.logs.GetLogs(filters.SubscriptionID(id))
	if !ok {
		return nil, fmt.Errorf("filter does not exist: %s", id)
	}

	result := make([]*RPCLog, len(logs))
	for i, metaLog := range logs {
		result[i] = NewRPCLog(metaLog.Log, metaLog.BlockId)
	}
	return result, nil
}
