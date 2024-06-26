package clickhouse

import (
	"context"
	"errors"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/NilFoundation/nil/common"
	"github.com/NilFoundation/nil/common/check"
	"github.com/NilFoundation/nil/core/types"
	"github.com/NilFoundation/nil/exporter"
)

type ClickhouseDriver struct {
	conn       driver.Conn
	insertConn driver.Conn
	options    clickhouse.Options
}

// I saw this trick. dunno should I use it here too
var (
	_ exporter.ExportDriver = &ClickhouseDriver{}
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

func (d *ClickhouseDriver) ExportBlocks(ctx context.Context, msgs []*exporter.BlockMsg) error {
	if err := exportMessagesAndLogs(ctx, d.insertConn, msgs); err != nil {
		return err
	}

	blockBatch, err := d.insertConn.PrepareBatch(ctx, "INSERT INTO blocks")
	if err != nil {
		return err
	}

	for _, block := range msgs {
		binary, blockErr := block.Block.MarshalSSZ()
		if blockErr != nil {
			return blockErr
		}
		binaryBlockExtended := &BlockWithBinary{
			Block:     *block.Block,
			Binary:    binary,
			ShardId:   block.Shard,
			Hash:      block.Block.Hash(),
			OutMsgNum: uint64(len(block.OutMessages)),
			InMsgNum:  uint64(len(block.InMessages)),
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

func exportMessagesAndLogs(ctx context.Context, conn driver.Conn, msgs []*exporter.BlockMsg) error {
	messageBatch, err := conn.PrepareBatch(ctx, "INSERT INTO messages")
	if err != nil {
		return err
	}
	receiptMap := make(map[common.Hash]*types.Receipt)
	for _, block := range msgs {
		for _, receipt := range block.Receipts {
			receiptMap[receipt.MsgHash] = receipt
		}
	}

	for _, block := range msgs {
		parentIndex := make([]common.Hash, len(block.OutMessages))
		for index, message := range block.InMessages {
			binary, messageErr := message.MarshalSSZ()
			if messageErr != nil {
				return messageErr
			}
			receipt, ok := receiptMap[message.Hash()]
			if !ok {
				return errors.New("receipt not found")
			}
			receiptBinary, err := receipt.MarshalSSZ()
			if err != nil {
				return err
			}
			for i := receipt.OutMsgIndex; i < receipt.OutMsgNum; i++ {
				parentIndex[i] = message.Hash()
			}
			binaryMessageExtended := &MessageWithBinary{
				Message:       *message,
				Binary:        binary,
				Success:       receipt.Success,
				GasUsed:       receipt.GasUsed,
				BlockId:       block.Block.Id,
				BlockHash:     block.Block.Hash(),
				Hash:          message.Hash(),
				ShardId:       block.Shard,
				ReceiptBinary: receiptBinary,
				MessageIndex:  types.MessageIndex(block.Positions[index]),
				Outgoing:      false,
				Timestamp:     block.Block.Timestamp,
			}
			messageErr = messageBatch.AppendStruct(binaryMessageExtended)
			if messageErr != nil {
				return fmt.Errorf("failed to append message to batch: %w", messageErr)
			}
		}
		for index, message := range block.OutMessages {
			binary, messageErr := message.MarshalSSZ()
			if messageErr != nil {
				return messageErr
			}
			binaryMessageExtended := &MessageWithBinary{
				Message:           *message,
				Binary:            binary,
				Success:           true,
				BlockId:           block.Block.Id,
				BlockHash:         block.Block.Hash(),
				Hash:              message.Hash(),
				ShardId:           block.Shard,
				ReceiptBinary:     make([]byte, 0),
				MessageIndex:      types.MessageIndex(index),
				Outgoing:          true,
				Timestamp:         block.Block.Timestamp,
				ParentTransaction: parentIndex[index],
			}
			messageErr = messageBatch.AppendStruct(binaryMessageExtended)
			if messageErr != nil {
				return fmt.Errorf("failed to append message to batch: %w", messageErr)
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

	for _, block := range msgs {
		for _, receipt := range block.Receipts {
			for _, log := range receipt.Logs {
				binary, logErr := log.MarshalSSZ()
				if logErr != nil {
					return logErr
				}
				logExtended := &LogWithBinary{
					MessageHash: receipt.MsgHash,
					Binary:      binary,
					Address:     log.Address,
					TopicsCount: uint8(len(log.Topics)),
					Data:        log.Data,
				}
				for i, topic := range log.Topics {
					switch i {
					case 0:
						logExtended.Topic1 = topic
					case 1:
						logExtended.Topic2 = topic
					case 2:
						logExtended.Topic3 = topic
					case 3:
						logExtended.Topic4 = topic
					}
				}
				logErr = logBatch.AppendStruct(logExtended)
				if logErr != nil {
					return fmt.Errorf("failed to append log to batch: %w", logErr)
				}
			}
		}
	}
	err = logBatch.Send()
	if err != nil {
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
