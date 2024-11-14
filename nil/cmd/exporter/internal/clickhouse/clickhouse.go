package clickhouse

import (
	"context"
	"errors"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/NilFoundation/nil/nil/cmd/exporter/internal"
	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/NilFoundation/nil/nil/common/ssz"
	"github.com/NilFoundation/nil/nil/internal/types"
)

type ClickhouseDriver struct {
	conn       driver.Conn
	insertConn driver.Conn
	options    clickhouse.Options
}

// I saw this trick. dunno should I use it here too
var (
	_ internal.ExportDriver = &ClickhouseDriver{}
)

// extend types.Block with binary field
type BlockWithBinary struct {
	types.Block
	Binary    []byte        `ch:"binary"`
	Hash      common.Hash   `ch:"hash"`
	ShardId   types.ShardId `ch:"shard_id"`
	OutMsgNum uint64        `ch:"out_msg_num"`
	InMsgNum  uint64        `ch:"in_msg_num"`
}

type MessageWithBinary struct {
	types.Message
	Success           bool               `ch:"success"`
	Status            string             `ch:"status"`
	GasUsed           types.Gas          `ch:"gas_used"`
	BlockId           types.BlockNumber  `ch:"block_id"`
	BlockHash         common.Hash        `ch:"block_hash"`
	Binary            []byte             `ch:"binary"`
	ReceiptBinary     []byte             `ch:"receipt_binary"`
	Hash              common.Hash        `ch:"hash"`
	ShardId           types.ShardId      `ch:"shard_id"`
	MessageIndex      types.MessageIndex `ch:"message_index"`
	Outgoing          bool               `ch:"outgoing"`
	Timestamp         uint64             `ch:"timestamp"`
	ParentTransaction common.Hash        `ch:"parent_transaction"`
	ErrorMessage      string             `ch:"error_message"`
}

func NewMessageWithBinary(
	message *types.Message,
	messageBinary ssz.SSZEncodedData,
	receipt *types.Receipt,
	receiptBinary ssz.SSZEncodedData,
	block *types.BlockWithExtractedData,
	idx types.MessageIndex,
	shardId types.ShardId,
) *MessageWithBinary {
	hash := message.Hash()
	res := &MessageWithBinary{
		Message:      *message,
		Binary:       messageBinary,
		BlockId:      block.Id,
		BlockHash:    block.Block.Hash(shardId),
		Hash:         hash,
		ShardId:      shardId,
		MessageIndex: idx,
		Timestamp:    block.Timestamp,
		ErrorMessage: block.Errors[hash],
	}
	if receipt != nil {
		res.Success = receipt.Success
		res.GasUsed = receipt.GasUsed
		res.ReceiptBinary = receiptBinary
	}
	return res
}

type LogWithBinary struct {
	MessageHash common.Hash   `ch:"message_hash"`
	Binary      []byte        `ch:"binary"`
	Address     types.Address `ch:"address"`
	TopicsCount uint8         `ch:"topics_count"`
	Topic1      common.Hash   `ch:"topic1"`
	Topic2      common.Hash   `ch:"topic2"`
	Topic3      common.Hash   `ch:"topic3"`
	Topic4      common.Hash   `ch:"topic4"`
	Data        []byte        `ch:"data"`
}

func NewLogWithBinary(log *types.Log, binary []byte, receipt *types.Receipt) *LogWithBinary {
	res := &LogWithBinary{
		MessageHash: receipt.MsgHash,
		Binary:      binary,
		Address:     log.Address,
		TopicsCount: uint8(len(log.Topics)),
		Data:        log.Data,
	}
	for i, topic := range log.Topics {
		switch i {
		case 0:
			res.Topic1 = topic
		case 1:
			res.Topic2 = topic
		case 2:
			res.Topic3 = topic
		case 3:
			res.Topic4 = topic
		}
	}
	return res
}

var tableSchemeCache map[string]reflectedScheme = nil

func initSchemeCache() map[string]reflectedScheme {
	tableScheme := make(map[string]reflectedScheme)

	blockScheme, err := reflectSchemeToClickhouse(&BlockWithBinary{})
	check.PanicIfErr(err)

	tableScheme["blocks"] = blockScheme
	messageScheme, err := reflectSchemeToClickhouse(&MessageWithBinary{})
	check.PanicIfErr(err)

	tableScheme["messages"] = messageScheme
	logScheme, err := reflectSchemeToClickhouse(&LogWithBinary{})
	check.PanicIfErr(err)

	tableScheme["logs"] = logScheme

	return tableScheme
}

func getTableScheme() map[string]reflectedScheme {
	if tableSchemeCache == nil {
		tableSchemeCache = initSchemeCache()
	}

	return tableSchemeCache
}

func NewClickhouseDriver(_ context.Context, endpoint, login, password, database string) (*ClickhouseDriver, error) {
	// Create connection to Clickhouse
	connectionOptions := clickhouse.Options{
		Auth: clickhouse.Auth{
			Username: login,
			Password: password,
			Database: database,
		},
		Addr: []string{endpoint},
	}
	conn, err := clickhouse.Open(&connectionOptions)
	if err != nil {
		return nil, err
	}

	insertConn, err := clickhouse.Open(&connectionOptions)
	if err != nil {
		return nil, err
	}
	return &ClickhouseDriver{
		conn:       conn,
		insertConn: insertConn,
		options:    connectionOptions,
	}, nil
}

func (d *ClickhouseDriver) Reconnect() error {
	var err error
	d.conn, err = clickhouse.Open(&d.options)
	if err != nil {
		return err
	}

	d.insertConn, err = clickhouse.Open(&d.options)
	return err
}

func (d *ClickhouseDriver) SetupScheme(ctx context.Context) error {
	return setupSchemeForClickhouse(ctx, d.conn)
}

func rowToBlock(rows driver.Rows) (*types.Block, error) {
	var binary []uint8
	if err := rows.Scan(&binary); err != nil {
		return nil, err
	}
	var block types.Block
	if err := block.UnmarshalSSZ(binary); err != nil {
		return nil, err
	}
	return &block, nil
}

func (d *ClickhouseDriver) FetchLatestProcessedBlock(ctx context.Context, shardId types.ShardId) (*types.Block, bool, error) {
	rows, err := d.conn.Query(ctx, `SELECT blocks.binary
from blocks
where shard_id = $1
order by blocks.id desc
limit 1`, shardId)
	if err != nil {
		return nil, false, err
	}
	defer func(rows driver.Rows) {
		_ = rows.Close()
	}(rows)
	if !rows.Next() {
		return nil, false, nil
	}
	block, err := rowToBlock(rows)
	if err != nil {
		return nil, false, err
	}
	return block, true, nil
}

func (d *ClickhouseDriver) FetchEarliestAbsentBlock(ctx context.Context, shardId types.ShardId) (types.BlockNumber, bool, error) {
	// join in clickhouse return default value on outer join
	rows, err := d.conn.Query(ctx, `SELECT a.id + 1
from blocks as a
		 left outer join blocks as b on a.id + 1 = b.id and a.shard_id = b.shard_id
where a.shard_id = $1 and a.id > b.id
order by a.id asc
`, shardId)
	if err != nil {
		return 0, false, err
	}
	defer func(rows driver.Rows) {
		_ = rows.Close()
	}(rows)
	if !rows.Next() {
		return 0, false, nil
	}
	var blockNumber uint64
	if err = rows.Scan(&blockNumber); err != nil {
		return 0, false, err
	}

	return types.BlockNumber(blockNumber), true, nil
}

func setupSchemeForClickhouse(ctx context.Context, conn driver.Conn) error {
	// Create table for blocks
	tableScheme := getTableScheme()
	blockScheme, ok := tableScheme["blocks"]
	if !ok {
		return errors.New("scheme for blocks not found")
	}

	err := conn.Exec(ctx, blockScheme.CreateTableQuery(
		"blocks",
		"ReplacingMergeTree",
		[]string{"shard_id", "hash"},
		[]string{"shard_id", "hash"},
	))
	if err != nil {
		return fmt.Errorf("failed to create table blocks: %w", err)
	}

	// Create table for messages
	messagesScheme, ok := tableScheme["messages"]
	if !ok {
		return errors.New("scheme for messages not found")
	}

	err = conn.Exec(
		ctx,
		messagesScheme.CreateTableQuery(
			"messages",
			"ReplacingMergeTree",
			[]string{"hash", "outgoing"},
			[]string{"hash", "outgoing"},
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create table messages: %w", err)
	}

	// Create table for receipts
	logScheme, ok := tableScheme["logs"]
	if !ok {
		return errors.New("scheme for receipts not found")
	}

	if err = conn.Exec(
		ctx, logScheme.CreateTableQuery(
			"logs",
			"ReplacingMergeTree",
			[]string{"message_hash"},
			[]string{"message_hash"},
		),
	); err != nil {
		return fmt.Errorf("failed to create table logs: %w", err)
	}

	return nil
}

type blockWithSSZ struct {
	decoded    *internal.BlockWithShardId
	sszEncoded *types.RawBlockWithExtractedData
}

type receiptWithSSZ struct {
	decoded    *types.Receipt
	sszEncoded ssz.SSZEncodedData
}

func (d *ClickhouseDriver) ExportBlocks(ctx context.Context, blocksToExport []*internal.BlockWithShardId) error {
	blocks := make([]blockWithSSZ, len(blocksToExport))
	receipts := make(map[common.Hash]receiptWithSSZ)
	for blockIndex, block := range blocksToExport {
		sszEncodedBlock, err := block.EncodeSSZ()
		if err != nil {
			return err
		}
		blocks[blockIndex] = blockWithSSZ{decoded: block, sszEncoded: sszEncodedBlock}
		for receiptIndex, receipt := range block.Receipts {
			receipts[receipt.MsgHash] = receiptWithSSZ{
				decoded:    receipt,
				sszEncoded: sszEncodedBlock.Receipts[receiptIndex],
			}
		}
	}

	if err := exportMessagesAndLogs(ctx, d.insertConn, blocks, receipts); err != nil {
		return err
	}

	blockBatch, err := d.insertConn.PrepareBatch(ctx, "INSERT INTO blocks")
	if err != nil {
		return err
	}

	for _, block := range blocks {
		binary, blockErr := block.decoded.MarshalSSZ()
		if blockErr != nil {
			return blockErr
		}
		binaryBlockExtended := &BlockWithBinary{
			Block:     *block.decoded.Block,
			Binary:    binary,
			ShardId:   block.decoded.ShardId,
			Hash:      block.decoded.Block.Hash(block.decoded.ShardId),
			OutMsgNum: uint64(len(block.decoded.OutMessages)),
			InMsgNum:  uint64(len(block.decoded.InMessages)),
		}
		blockErr = blockBatch.AppendStruct(binaryBlockExtended)
		if blockErr != nil {
			return fmt.Errorf("failed to append block to batch: %w", blockErr)
		}
	}

	err = blockBatch.Send()
	if err != nil {
		return err
	}

	return nil
}

func exportMessagesAndLogs(ctx context.Context, conn driver.Conn, blocks []blockWithSSZ, receipts map[common.Hash]receiptWithSSZ) error {
	messageBatch, err := conn.PrepareBatch(ctx, "INSERT INTO messages")
	if err != nil {
		return err
	}

	for _, block := range blocks {
		parentIndex := make([]common.Hash, len(block.decoded.OutMessages))
		for inMsgIndex, message := range block.decoded.InMessages {
			hash := message.Hash()
			receipt, ok := receipts[hash]
			if !ok {
				return errors.New("receipt not found")
			}
			for i := receipt.decoded.OutMsgIndex; i < receipt.decoded.OutMsgIndex+receipt.decoded.OutMsgNum; i++ {
				parentIndex[i] = hash
			}
			mb := NewMessageWithBinary(
				message,
				block.sszEncoded.InMessages[inMsgIndex],
				receipt.decoded,
				receipt.sszEncoded,
				block.decoded.BlockWithExtractedData,
				types.MessageIndex(inMsgIndex),
				block.decoded.ShardId)
			if err := messageBatch.AppendStruct(mb); err != nil {
				return fmt.Errorf("failed to append message to batch: %w", err)
			}
		}
		for outMessageIndex, message := range block.decoded.OutMessages {
			mb := NewMessageWithBinary(
				message,
				block.sszEncoded.OutMessages[outMessageIndex],
				nil,
				nil,
				block.decoded.BlockWithExtractedData,
				types.MessageIndex(outMessageIndex),
				block.decoded.ShardId)
			mb.Outgoing = true
			mb.ParentTransaction = parentIndex[outMessageIndex]
			if err := messageBatch.AppendStruct(mb); err != nil {
				return fmt.Errorf("failed to append message to batch: %w", err)
			}
		}
	}

	err = messageBatch.Send()
	if err != nil {
		return fmt.Errorf("failed to send messages batch: %w", err)
	}

	logBatch, err := conn.PrepareBatch(ctx, "INSERT INTO logs")
	if err != nil {
		return fmt.Errorf("failed to prepare log batch: %w", err)
	}

	for _, block := range blocks {
		for _, receipt := range block.decoded.Receipts {
			for _, log := range receipt.Logs {
				binary, logErr := log.MarshalSSZ()
				if logErr != nil {
					return logErr
				}
				if err := logBatch.AppendStruct(NewLogWithBinary(log, binary, receipt)); err != nil {
					return fmt.Errorf("failed to append log to batch: %w", err)
				}
			}
		}
	}

	if err = logBatch.Send(); err != nil {
		return fmt.Errorf("failed to send logs batch: %w", err)
	}
	return nil
}

func (d *ClickhouseDriver) FetchBlock(ctx context.Context, id types.ShardId, number types.BlockNumber) (*types.Block, bool, error) {
	rows, err := d.conn.Query(ctx, "SELECT binary FROM blocks WHERE shard_id = $1 AND id = $2", id, number)
	if err != nil {
		return nil, false, err
	}
	if !rows.Next() {
		return nil, false, nil
	}
	block, err := rowToBlock(rows)
	if err != nil {
		return nil, false, err
	}
	return block, true, nil
}

func (d *ClickhouseDriver) FetchNextPresentBlock(ctx context.Context, shardId types.ShardId, number types.BlockNumber) (types.BlockNumber, bool, error) {
	rows, err := d.conn.Query(ctx, `SELECT blocks.id
from blocks
where shard_id = $1 and id > $2 order by id asc`, shardId, number)
	if err != nil {
		return 0, false, err
	}
	defer func(rows driver.Rows) {
		_ = rows.Close()
	}(rows)
	if !rows.Next() {
		return 0, false, nil
	}
	var blockNumber uint64
	if err := rows.Scan(&blockNumber); err != nil {
		return 0, false, err
	}
	return types.BlockNumber(blockNumber), true, nil
}
