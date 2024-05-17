package filters

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/core/db"
	"github.com/NilFoundation/nil/core/mpt"
	"github.com/NilFoundation/nil/core/types"
	"github.com/holiman/uint256"
)

var logger = common.NewLogger("filters", false)

type Filter struct {
	query  *FilterQuery
	output chan *types.Log
}

// FilterQuery contains options for contract log filtering.
type FilterQuery struct {
	FromBlock *uint256.Int    // beginning of the queried range, nil means genesis block
	ToBlock   *uint256.Int    // end of the range, nil means latest block
	Address   *common.Address // restricts matches to events created by specific contracts

	// The Topic list restricts matches to particular event topics. Each event has a list
	// of topics. Topics matches a prefix of that list. An empty element slice matches any
	// topic. Non-empty elements represent an alternative that matches any of the
	// contained topics.
	//
	// Examples:
	// {} or nil          matches any topic list
	// {{A}}              matches topic A in first position
	// {{}, {B}}          matches any topic in first position AND B in second position
	// {{A}, {B}}         matches topic A in first position AND B in second position
	// {{A, B}, {C, D}}   matches topic (A OR B) in first position AND (C OR D) in second position
	Topics [][]common.Hash
}

type (
	SubscriptionID string
)

type FiltersManager struct {
	ctx      context.Context
	db       db.DB
	shardId  types.ShardId
	filters  map[SubscriptionID]*Filter
	mutex    sync.RWMutex
	lastHash common.Hash
}

func NewFiltersManager(ctx context.Context, db db.DB, noPolling bool) *FiltersManager {
	f := &FiltersManager{
		ctx:      ctx,
		db:       db,
		filters:  make(map[SubscriptionID]*Filter),
		lastHash: common.EmptyHash,
	}

	if !noPolling {
		go f.PollBlocks(200)
	}

	return f
}

func (f *Filter) LogsChannel() <-chan *types.Log {
	return f.output
}

func (m *FiltersManager) NewFilter(query *FilterQuery) (SubscriptionID, *Filter) {
	id := generateSubscriptionID()

	m.mutex.Lock()
	defer m.mutex.Unlock()

	filter := Filter{query: query, output: make(chan *types.Log, 100)}
	m.filters[id] = &filter

	return id, &filter
}

func (m *FiltersManager) RemoveFilter(id SubscriptionID) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	filter, ok := m.filters[id]
	if ok {
		close(filter.output)
		delete(m.filters, id)
		return true
	}
	return false
}

// PollBlocks polls the blockchain for new committed blocks, if found - parse it's receipts and send logs to the matched
// filters. TODO: Remove polling, probably blockhain should raise events about new blocks by itself.
func (m *FiltersManager) PollBlocks(delay time.Duration) {
	for {
		select {
		case <-m.ctx.Done():
			return
		default:
		}
		time.Sleep(delay * time.Millisecond)

		lastHash, err := m.getLastBlockHash()
		if err != nil {
			logger.Warn().Msgf("getLastBlockHash failed: %s", err)
			continue
		} else if lastHash == common.EmptyHash {
			continue
		}

		if m.lastHash != lastHash {
			if err := m.processBlockHash(&lastHash); err != nil {
				logger.Warn().Msgf("processBlock failed: %s", err)
				continue
			}
			m.lastHash = lastHash
		}
	}
}

func (m *FiltersManager) processBlockHash(lastHash *common.Hash) error {
	tx, err := m.db.CreateRoTx(m.ctx)
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}
	block := db.ReadBlock(tx, *lastHash)
	if block == nil {
		return fmt.Errorf("Can not read last block")
	}

	// TODO: We should process missing blocks by unwinding blocks chain.
	if block.PrevBlock != m.lastHash {
		logger.Warn().Msgf("block prev hash does not match last block hash")
	}

	var receipts types.Receipts

	mptReceipts := mpt.NewMerklePatriciaTrieWithRoot(m.db, db.ReceiptTrieTableName(m.shardId), block.ReceiptsRoot)
	for kv := range mptReceipts.Iterate() {
		receipt := types.Receipt{}
		if err := receipt.UnmarshalSSZ(kv.Value); err != nil {
			return err
		}
		receipts = append(receipts, &receipt)
	}

	return m.process(block, receipts)
}

func (m *FiltersManager) process(block *types.Block, receipts types.Receipts) error {
	for _, filter := range m.filters {
		for _, receipt := range receipts {
			if filter.query.Address != nil && *filter.query.Address != receipt.ContractAddress {
				continue
			}
			for _, log := range receipt.Logs {
				found := true
				for i, topics := range filter.query.Topics {
					if i >= log.TopicsNum() {
						found = false
						break
					}
					switch len(topics) {
					case 0:
						continue
					case 1: // valid case, process after switch block
					default:
						panic("TODO: Topics disjunction isn't supported yet")
					}

					logTopic := log.Topics[i]
					if logTopic != topics[0] {
						found = false
						break
					}
				}
				if found {
					filter.output <- log
				}
			}
		}
	}

	return nil
}

func (m *FiltersManager) OnNewBlock(block *types.Block) {
}

func (m *FiltersManager) getLastBlockHash() (common.Hash, error) {
	lastBlockRaw, err := m.db.Get(db.LastBlockTable, []byte(strconv.Itoa(int(m.shardId))))
	if err == nil && lastBlockRaw != nil {
		return common.Hash(*lastBlockRaw), nil
	}
	return common.EmptyHash, err
}

var globalSubscriptionId uint64

func generateSubscriptionID() SubscriptionID {
	id := [16]byte{}
	sb := new(strings.Builder)
	hex := hex.NewEncoder(sb)
	binary.LittleEndian.PutUint64(id[:], atomic.AddUint64(&globalSubscriptionId, 1))
	// Try 4 times to generate an id
	for i := 0; i < 4; i++ {
		_, err := rand.Read(id[8:])
		if err == nil {
			break
		}
	}
	// If the computer has no functioning secure rand source, it will just use the incrementing number
	if _, err := hex.Write(id[:]); err != nil {
		return ""
	}
	return SubscriptionID(sb.String())
}
