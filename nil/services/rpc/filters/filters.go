package filters

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/execution"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/rpc/transport"
	"github.com/holiman/uint256"
)

var logger = logging.NewLogger("filters")

type MetaLog struct {
	Log     *types.Log
	BlockId types.BlockNumber
}

type Filter struct {
	query  *FilterQuery
	output chan *MetaLog
}

// FilterQuery contains options for contract log filtering.
type FilterQuery struct {
	BlockHash *common.Hash    // used by eth_getLogs, return logs only from block with this hash
	FromBlock *uint256.Int    // beginning of the queried range, nil means genesis block
	ToBlock   *uint256.Int    // end of the range, nil means the latest block
	Addresses []types.Address // restricts matches to events created by specific contracts

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
	ctx       context.Context
	db        db.ReadOnlyDB
	shardId   types.ShardId
	filters   map[SubscriptionID]*Filter
	blockSubs map[SubscriptionID]chan<- *types.Block
	mutex     sync.RWMutex
	lastHash  common.Hash
	wg        sync.WaitGroup
}

func NewFiltersManager(ctx context.Context, db db.ReadOnlyDB, noPolling bool) *FiltersManager {
	f := &FiltersManager{
		ctx:       ctx,
		db:        db,
		filters:   make(map[SubscriptionID]*Filter),
		blockSubs: make(map[SubscriptionID]chan<- *types.Block),
		lastHash:  common.EmptyHash,
	}

	if !noPolling {
		f.wg.Add(1)
		go f.PollBlocks(200 * time.Millisecond)
	}

	return f
}

func (f *FiltersManager) WaitForShutdown() {
	f.wg.Wait()
}

func (f *Filter) LogsChannel() <-chan *MetaLog {
	return f.output
}

func (m *FiltersManager) NewFilter(query *FilterQuery) (SubscriptionID, *Filter) {
	id := generateSubscriptionID()

	m.mutex.Lock()
	defer m.mutex.Unlock()

	filter := &Filter{query: query, output: make(chan *MetaLog, 100)}
	m.filters[id] = filter

	if query.FromBlock != nil || query.ToBlock != nil {
		if err := m.processBlocksRange(filter); err != nil {
			logger.Error().Err(err).Msg("Filter processing blocks failed")
			return "", nil
		}
	}

	return id, filter
}

func (m *FiltersManager) RemoveFilter(id SubscriptionID) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	filter, exist := m.filters[id]
	if exist {
		close(filter.output)
		delete(m.filters, id)
	}
	return exist
}

func (m *FiltersManager) AddBlocksListener() (SubscriptionID, <-chan *types.Block) {
	id := generateSubscriptionID()
	ch := make(chan *types.Block, 100)

	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.blockSubs[id] = ch
	return id, ch
}

func (m *FiltersManager) RemoveBlocksListener(id SubscriptionID) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	_, exist := m.blockSubs[id]
	if exist {
		close(m.blockSubs[id])
		delete(m.blockSubs, id)
	}
	return exist
}

// PollBlocks polls the blockchain for new committed blocks, if found - parse it's receipts and send logs to the matched
// filters. TODO: Remove polling, probably blockhain should raise events about new blocks by itself.
func (m *FiltersManager) PollBlocks(delay time.Duration) {
	defer m.wg.Done()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-time.After(delay):
		}

		lastHash, err := m.getLastBlockHash()
		if err != nil {
			if !errors.Is(err, db.ErrKeyNotFound) {
				logger.Warn().Err(err).Msg("getLastBlockHash failed")
			}
			continue
		}

		if m.lastHash != lastHash {
			m.mutex.Lock()
			for currHash := lastHash; m.lastHash != currHash; {
				block, err := m.processBlockHash(currHash)
				if err != nil {
					logger.Warn().Err(err).Msg("processBlockHash failed")
					continue
				}
				for _, ch := range m.blockSubs {
					// Don't send if the channel is full. Probably subscriber just disconnected, and it shouldn't block us.
					if len(ch) < cap(ch) {
						ch <- block
					}
				}
				currHash = block.PrevBlock
				if currHash == common.EmptyHash {
					break
				}
			}
			m.lastHash = lastHash
			m.mutex.Unlock()
		}
	}
}

// / If FromBlock is set in the filter, then processBlocksRange processes all blocks in the range [FromBlock..ToBlock].
func (m *FiltersManager) processBlocksRange(filter *Filter) error {
	tx, err := m.db.CreateRoTx(m.ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var fromBlockNum, lastBlockNum uint64

	if filter.query.FromBlock != nil {
		fromBlockNum = filter.query.FromBlock.Uint64()
	} else {
		fromBlockNum = 0
	}

	if filter.query.ToBlock != nil {
		lastBlockNum = filter.query.ToBlock.Uint64()
	} else {
		lastBlock, _, err := db.ReadLastBlock(tx, m.shardId)
		if err != nil {
			return err
		}
		lastBlockNum = uint64(lastBlock.Id)
	}

	for ; fromBlockNum <= lastBlockNum; fromBlockNum++ {
		block, err := db.ReadBlockByNumber(tx, m.shardId, types.BlockNumber(fromBlockNum))
		if err != nil {
			return err
		}
		receipts, err := m.readReceipts(tx, block)
		if err != nil {
			return err
		}
		if err = m.processFilter(block, filter, receipts); err != nil {
			return err
		}
	}
	return nil
}

func (m *FiltersManager) readReceipts(tx db.RoTx, block *types.Block) ([]*types.Receipt, error) {
	reader := execution.NewDbReceiptTrieReader(tx, m.shardId)
	reader.SetRootHash(block.ReceiptsRoot)
	return reader.Values()
}

func (m *FiltersManager) processBlockHash(lastHash common.Hash) (*types.Block, error) {
	tx, err := m.db.CreateRoTx(m.ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	block, err := db.ReadBlock(tx, m.shardId, lastHash)
	if err != nil {
		return nil, err
	}

	receipts, err := m.readReceipts(tx, block)
	if err != nil {
		return nil, err
	}

	return block, m.process(block, receipts)
}

func (m *FiltersManager) processFilter(block *types.Block, filter *Filter, receipts types.Receipts) error {
	if filter.query.ToBlock != nil && uint64(block.Id) > filter.query.ToBlock.Uint64() {
		return nil
	}
	for _, receipt := range receipts {
		if len(filter.query.Addresses) != 0 && !slices.Contains(filter.query.Addresses, receipt.ContractAddress) {
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
				filter.output <- &MetaLog{log, block.Id}
			}
		}
	}
	return nil
}

func (m *FiltersManager) process(block *types.Block, receipts types.Receipts) error {
	for _, filter := range m.filters {
		err := m.processFilter(block, filter, receipts)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *FiltersManager) OnNewBlock(block *types.Block) {
}

func (m *FiltersManager) getLastBlockHash() (common.Hash, error) {
	tx, err := m.db.CreateRoTx(m.ctx)
	if err != nil {
		return common.Hash{}, err
	}
	defer tx.Rollback()

	return db.ReadLastBlockHash(tx, m.shardId)
}

var globalSubscriptionId uint64

func generateSubscriptionID() SubscriptionID {
	id := [16]byte{}
	sb := new(strings.Builder)
	h := hex.NewEncoder(sb)
	binary.LittleEndian.PutUint64(id[:], atomic.AddUint64(&globalSubscriptionId, 1))
	// Try 4 times to generate an id
	for range 4 {
		_, err := rand.Read(id[8:])
		if err == nil {
			break
		}
	}
	// If the computer has no functioning secure rand source, it will just use the incrementing number
	if _, err := h.Write(id[:]); err != nil {
		return ""
	}
	return SubscriptionID(sb.String())
}

func (args *FilterQuery) UnmarshalJSON(data []byte) error {
	type input struct {
		BlockHash *common.Hash           `json:"blockHash"`
		FromBlock *transport.BlockNumber `json:"fromBlock"`
		ToBlock   *transport.BlockNumber `json:"toBlock"`
		Addresses interface{}            `json:"address"`
		Topics    []interface{}          `json:"topics"`
	}

	var raw input
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if raw.BlockHash != nil {
		if raw.FromBlock != nil || raw.ToBlock != nil {
			// BlockHash is mutually exclusive with FromBlock/ToBlock criteria
			return errors.New("cannot specify both BlockHash and FromBlock/ToBlock, choose one or the other")
		}
		args.BlockHash = raw.BlockHash
	} else {
		if raw.FromBlock != nil {
			args.FromBlock = uint256.NewInt(raw.FromBlock.Uint64())
		}

		if raw.ToBlock != nil {
			args.ToBlock = uint256.NewInt(raw.ToBlock.Uint64())
		}
	}

	args.Addresses = []types.Address{}

	if raw.Addresses != nil {
		// raw.Address can contain a single address or an array of addresses
		switch rawAddr := raw.Addresses.(type) {
		case []interface{}:
			for i, addr := range rawAddr {
				if strAddr, ok := addr.(string); ok {
					addr, err := decodeAddress(strAddr)
					if err != nil {
						return fmt.Errorf("invalid address at index %d: %w", i, err)
					}
					args.Addresses = append(args.Addresses, addr)
				} else {
					return fmt.Errorf("non-string address at index %d", i)
				}
			}
		case string:
			addr, err := decodeAddress(rawAddr)
			if err != nil {
				return fmt.Errorf("invalid address: %w", err)
			}
			args.Addresses = []types.Address{addr}
		default:
			return errors.New("invalid addresses in query")
		}
	}

	// topics is an array consisting of strings and/or arrays of strings.
	// JSON null values are converted to common.Hash{} and ignored by the filter manager.
	if len(raw.Topics) > 0 {
		args.Topics = make([][]common.Hash, len(raw.Topics))
		for i, t := range raw.Topics {
			switch topic := t.(type) {
			case nil:
				// ignore topic when matching logs

			case string:
				// match specific topic
				top, err := decodeTopic(topic)
				if err != nil {
					return err
				}
				args.Topics[i] = []common.Hash{top}

			case []interface{}:
				// or case e.g. [null, "topic0", "topic1"]
				for _, rawTopic := range topic {
					if rawTopic == nil {
						// null component, match all
						args.Topics[i] = nil
						break
					}
					if topic, ok := rawTopic.(string); ok {
						parsed, err := decodeTopic(topic)
						if err != nil {
							return err
						}
						args.Topics[i] = append(args.Topics[i], parsed)
					} else {
						return errors.New("invalid topic(s)")
					}
				}
			default:
				return errors.New("invalid topic(s)")
			}
		}
	}

	return nil
}

func decodeAddress(s string) (types.Address, error) {
	b, err := hexutil.Decode(s)
	if err == nil && len(b) != types.AddrSize {
		err = fmt.Errorf("hex has invalid length %d after decoding; expected %d for address", len(b), types.AddrSize)
	}
	return types.BytesToAddress(b), err
}

func decodeTopic(s string) (common.Hash, error) {
	b, err := hexutil.Decode(s)
	if err == nil && len(b) != common.HashSize {
		err = fmt.Errorf("hex has invalid length %d after decoding; expected %d for topic", len(b), common.HashSize)
	}
	return common.BytesToHash(b), err
}
