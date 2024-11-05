package debug

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/NilFoundation/nil/nil/cmd/nil/internal/common"
	libcommon "github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/NilFoundation/nil/nil/services/cometa"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

const invalidPc = ^uint64(0)

var logger = logging.NewLogger("debugCommand")

type DebugHandler struct {
	Service        *cliservice.Service
	CometaClient   *cometa.Client
	RootReceipt    *ReceiptInfo
	MsgHash        libcommon.Hash
	contractsCache map[types.Address]*cometa.Contract
	messageCache   map[libcommon.Hash]*jsonrpc.RPCInMessage
}

type ReceiptInfo struct {
	Index       int
	Receipt     *jsonrpc.RPCReceipt
	Message     *jsonrpc.RPCInMessage
	Contract    *cometa.Contract
	OutReceipts []*ReceiptInfo
}

func GetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "debug [options] message hash",
		Short: "Debug a message",
		Args:  cobra.ExactArgs(1),
		RunE:  runCommand,
	}

	cmd.Flags().Uint64("pc", invalidPc, "Specify the Program Counter to debug")

	return cmd
}

func NewDebugHandler(service *cliservice.Service, cometaClient *cometa.Client, msgHash libcommon.Hash) *DebugHandler {
	return &DebugHandler{
		Service:        service,
		CometaClient:   cometaClient,
		MsgHash:        msgHash,
		contractsCache: make(map[types.Address]*cometa.Contract),
		messageCache:   make(map[libcommon.Hash]*jsonrpc.RPCInMessage),
	}
}

func (d *DebugHandler) GetContract(address types.Address) (*cometa.Contract, error) {
	contract, ok := d.contractsCache[address]
	if ok {
		return contract, nil
	}
	contractData, err := d.CometaClient.GetContract(address)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch the contract data: %w", err)
	}
	contract, err = cometa.NewContractFromData(contractData)
	if err != nil {
		return nil, fmt.Errorf("failed to create a contract from the data: %w", err)
	}
	d.contractsCache[address] = contract
	return contract, nil
}

func (d *DebugHandler) GetMessage(receipt *jsonrpc.RPCReceipt) (*jsonrpc.RPCInMessage, error) {
	msg, ok := d.messageCache[receipt.MsgHash]
	if ok {
		return msg, nil
	}
	msg, err := d.Service.FetchMessageByHash(receipt.ContractAddress.ShardId(), receipt.MsgHash)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch the contract data: %w", err)
	}
	d.messageCache[receipt.MsgHash] = msg
	return msg, nil
}

var msgIndex = 0

func (d *DebugHandler) CollectReceipts(rootReceipt *jsonrpc.RPCReceipt) error {
	msgIndex = 0
	var err error
	d.RootReceipt, err = d.CollectReceiptsRec(nil, rootReceipt)
	if err != nil {
		return fmt.Errorf("failed to collect receipts: %w", err)
	}
	return nil
}

func (d *DebugHandler) CollectReceiptsRec(parentReceipt *ReceiptInfo, receipt *jsonrpc.RPCReceipt) (*ReceiptInfo, error) {
	contract, err := d.GetContract(receipt.ContractAddress)
	if err != nil {
		contract = nil
	}
	msg, err := d.GetMessage(receipt)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch a message: %w", err)
	}
	receiptInfo := &ReceiptInfo{
		Index:    msgIndex,
		Receipt:  receipt,
		Message:  msg,
		Contract: contract,
	}
	msgIndex += 1
	for _, outReceipt := range receipt.OutReceipts {
		if _, err = d.CollectReceiptsRec(receiptInfo, outReceipt); err != nil {
			return nil, err
		}
	}
	if parentReceipt != nil {
		parentReceipt.OutReceipts = append(parentReceipt.OutReceipts, receiptInfo)
	}
	return receiptInfo, nil
}

func (d *DebugHandler) SelectFailedReceipts() []*ReceiptInfo {
	resList := make([]*ReceiptInfo, 0, 8)
	workList := make([]*ReceiptInfo, 0, 16)
	workList = append(workList, d.RootReceipt)

	for len(workList) > 0 {
		receipt := workList[0]
		workList = workList[1:]
		if !receipt.Receipt.Success {
			resList = append(resList, receipt)
		}
		workList = append(workList, receipt.OutReceipts...)
	}
	return resList
}

func (d *DebugHandler) PrintSourceLocation(receipt *ReceiptInfo, loc *cometa.LineLocation) error {
	lines, err := receipt.Contract.GetSourceLines(loc.FileName)
	if err != nil {
		return fmt.Errorf("Failed to fetch the source lines: %w\n", err)
	}
	startLine := int(loc.Line) - 3
	if startLine < 1 {
		startLine = 1
	}
	endLine := int(loc.Line) + 3
	if endLine >= len(lines) {
		endLine = len(lines)
	}
	length := loc.Length
	if (uint(len(lines[loc.Line-1])) - loc.Column) < length {
		length = uint(len(lines[loc.Line-1])) - loc.Column + 1
	}
	fmt.Printf("Failed location for the message #%d: %s\n", receipt.Index, color.RedString(loc.String()))
	for i := startLine; i <= endLine; i++ {
		fmt.Printf("%5d: %s\n", i, lines[i-1])
		if i == int(loc.Line) {
			for range int(loc.Column) + 6 {
				fmt.Printf(" ")
			}
			for range int(length) {
				fmt.Print(color.RedString("^"))
			}
			fmt.Println("")
		}
	}
	return nil
}

func (d *DebugHandler) ShowFailures() {
	failedReceipts := d.SelectFailedReceipts()

	for _, receipt := range failedReceipts {
		if receipt.Receipt.FailedPc == 0 {
			continue
		}
		if receipt.Contract == nil {
			color.Red("Failed to get a contract for the message #%d\n", receipt.Index)
			continue
		}
		loc, err := receipt.Contract.GetLineLocation(receipt.Receipt.FailedPc)
		if err != nil {
			color.Red("Failed to fetch the location: %v\n", err)
		} else if err = d.PrintSourceLocation(receipt, loc); err != nil {
			color.Red("Failed to print the source location: %v", err)
		}
	}
}

var (
	keyColor      = color.New(color.FgCyan)
	unknownColor  = color.New(color.FgRed)
	calldataColor = color.New(color.FgMagenta)
)

func (d *DebugHandler) PrintReceipt(receipt *ReceiptInfo, indentEntry, indent string) {
	name := unknownColor.Sprint("unknown")
	hasContract := receipt.Contract != nil
	if hasContract {
		name = receipt.Contract.ShortName()
	}

	makeKey := func(key string) string {
		key = keyColor.Sprint(key)
		return fmt.Sprintf("%s%-20s: ", indent, key)
	}

	makeKeyEntry := func(key string) string {
		key = keyColor.Sprint(key)
		return fmt.Sprintf("%s%-20s: ", indentEntry, key)
	}

	flags := receipt.Message.Flags.String()
	if receipt.Message.RequestId != 0 && !receipt.Message.Flags.IsResponse() {
		flags += ", Request"
	}

	fmt.Printf("%s0x%x\n", makeKeyEntry("Message"), receipt.Message.Hash)
	fmt.Printf("%s%s\n", makeKey("Contract"), color.MagentaString(name))
	fmt.Printf("%s%s\n", makeKey("Flags"), flags)
	fmt.Printf("%s%s\n", makeKey("Address"), receipt.Receipt.ContractAddress.Hex())
	if hasContract && !receipt.Message.Flags.GetBit(types.MessageFlagResponse) {
		calldata, err := receipt.Contract.DecodeCallData(receipt.Message.Data)
		if err != nil {
			errStr := color.RedString("Failed to decode: %s", err.Error())
			fmt.Printf("%s[%s]%s\n", makeKey("CallData"), errStr, types.Code(receipt.Message.Data).Hex())
		} else {
			fmt.Printf("%s%s\n", makeKey("CallData"), calldataColor.Sprint(calldata))
		}
	} else if len(receipt.Message.Data) != 0 {
		fmt.Printf("%s%s\n", makeKey("CallData"), types.Code(receipt.Message.Data).Hex())
	}
	if !receipt.Receipt.Success {
		fmt.Printf("%s%s\n", makeKey("Status"), color.RedString(receipt.Receipt.Status))
		fmt.Printf("%s%d\n", makeKey("FailedPc"), receipt.Receipt.FailedPc)
	} else {
		fmt.Printf("%s%s\n", makeKey("Status"), color.GreenString(receipt.Receipt.Status))
	}
	if !receipt.Message.Flags.GetBit(types.MessageFlagRefund) {
		fmt.Printf("%s%d\n", makeKey("GasUsed"), receipt.Receipt.GasUsed)
	}
	fmt.Printf("%s%d\n", makeKey("RequestId"), receipt.Message.RequestId)
	fmt.Printf("%s%d:%d\n", makeKey("Block"), receipt.Receipt.ContractAddress.ShardId(), receipt.Message.BlockNumber)

	if len(receipt.OutReceipts) > 0 {
		for i, outReceipt := range receipt.OutReceipts {
			if i == len(receipt.OutReceipts)-1 {
				indentEntry = indent + "\u2514 " // `└` symbol
			} else {
				indentEntry = indent + "\u251c " // `├` symbol
			}
			var indent2 string
			if i < len(receipt.OutReceipts)-1 {
				indent2 = indent + "\u2502 " // `│` symbol
			} else {
				indent2 = indent + "  "
			}

			d.PrintReceipt(outReceipt, indentEntry, indent2)
		}
	}
}

func (d *DebugHandler) PrintMessageChain() {
	d.PrintReceipt(d.RootReceipt, "", "")
}

func runCommand(_ *cobra.Command, args []string) error {
	service := cliservice.NewService(common.GetRpcClient(), nil)

	hashStr := args[0]
	shardId := types.BaseShardId

	parts := strings.Split(hashStr, ":")
	if len(parts) > 1 {
		shardIdUint, err := strconv.ParseUint(parts[0], 10, 64)
		if err != nil {
			return err
		}
		shardId = types.ShardId(shardIdUint)
		hashStr = parts[1]
	}

	var msgHash libcommon.Hash
	if err := msgHash.Set(hashStr); err != nil {
		return err
	}
	if msgHash == libcommon.EmptyHash {
		return errors.New("empty msgHash")
	}

	cometa := common.GetCometaRpcClient()

	debugHandler := NewDebugHandler(service, cometa, msgHash)

	receipt, err := service.FetchReceiptByHash(shardId, msgHash)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to fetch the receipt")
		return err
	}
	if receipt == nil {
		return errors.New("no receipt found for the message")
	}

	if err = debugHandler.CollectReceipts(receipt); err != nil {
		logger.Error().Err(err).Msg("Failed to collect the receipts")
		return err
	}

	debugHandler.PrintMessageChain()

	fmt.Println()

	debugHandler.ShowFailures()

	return nil
}
