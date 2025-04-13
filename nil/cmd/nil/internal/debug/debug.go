package debug

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NilFoundation/nil/nil/cmd/nil/common"
	libcommon "github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/logging"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/services/cliservice"
	"github.com/NilFoundation/nil/nil/services/cometa"
	"github.com/NilFoundation/nil/nil/services/rpc/jsonrpc"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var logger = logging.NewLogger("debugCommand")

type debugParams struct {
	fullOutput bool
}

type DebugHandler struct {
	params           *debugParams
	Service          *cliservice.Service
	CometaClient     *cometa.Client
	RootReceipt      *ReceiptInfo
	TxnHash          libcommon.Hash
	contractsCache   map[types.Address]*cometa.Contract
	transactionCache map[libcommon.Hash]*jsonrpc.RPCInTransaction
}

type ReceiptInfo struct {
	Index       int
	Receipt     *jsonrpc.RPCReceipt
	Transaction *jsonrpc.RPCInTransaction
	Contract    *cometa.Contract
	OutReceipts []*ReceiptInfo
}

func GetCommand() *cobra.Command {
	params := &debugParams{}

	cmd := &cobra.Command{
		Use:   "debug [options] transaction hash",
		Short: "Debug a transaction",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCommand(cmd, args, params)
		},
	}

	cmd.Flags().BoolVar(&params.fullOutput, "full", false, "Show full data output(don't truncate big data)")

	return cmd
}

func NewDebugHandler(
	params *debugParams,
	service *cliservice.Service,
	cometaClient *cometa.Client,
	txnHash libcommon.Hash,
) *DebugHandler {
	return &DebugHandler{
		params:           params,
		Service:          service,
		CometaClient:     cometaClient,
		TxnHash:          txnHash,
		contractsCache:   make(map[types.Address]*cometa.Contract),
		transactionCache: make(map[libcommon.Hash]*jsonrpc.RPCInTransaction),
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

func (d *DebugHandler) GetTransaction(receipt *jsonrpc.RPCReceipt) (*jsonrpc.RPCInTransaction, error) {
	txn, ok := d.transactionCache[receipt.TxnHash]
	if ok {
		return txn, nil
	}
	txn, err := d.Service.FetchTransactionByHash(receipt.TxnHash)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch the contract data: %w", err)
	}
	d.transactionCache[receipt.TxnHash] = txn
	return txn, nil
}

var txnIndex = 0

func (d *DebugHandler) CollectReceipts(rootReceipt *jsonrpc.RPCReceipt) error {
	txnIndex = 0
	var err error
	d.RootReceipt, err = d.CollectReceiptsRec(nil, rootReceipt)
	if err != nil {
		return fmt.Errorf("failed to collect receipts: %w", err)
	}
	return nil
}

func (d *DebugHandler) CollectReceiptsRec(
	parentReceipt *ReceiptInfo,
	receipt *jsonrpc.RPCReceipt,
) (*ReceiptInfo, error) {
	contract, err := d.GetContract(receipt.ContractAddress)
	if err != nil {
		contract = nil
	}
	txn, err := d.GetTransaction(receipt)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch a transaction: %w", err)
	}
	receiptInfo := &ReceiptInfo{
		Index:       txnIndex,
		Receipt:     receipt,
		Transaction: txn,
		Contract:    contract,
	}
	txnIndex++
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

func (d *DebugHandler) PrintSourceLocation(receipt *ReceiptInfo, loc *cometa.Location) error {
	lines, err := receipt.Contract.GetSourceLines(loc.FileName)
	if err != nil {
		return fmt.Errorf("failed to fetch the source lines: %w", err)
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
	fmt.Printf("Failed location for the transaction #%d: %s\n", receipt.Index, color.RedString(loc.String()))
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
			color.Red("Failed to get a contract for the transaction #%d\n", receipt.Index)
			continue
		}
		loc, err := receipt.Contract.GetLocation(receipt.Receipt.FailedPc)
		if err != nil {
			color.Red("Failed to fetch the location: %v\n", err)
		} else if err = d.PrintSourceLocation(receipt, loc); err != nil {
			color.Red("Failed to print the source location: %v", err)
		}
	}
}

var (
	keyColor      = color.New(color.FgCyan)
	calldataColor = color.New(color.FgMagenta)
	logsColor     = color.New(color.FgMagenta)
)

func (d *DebugHandler) truncateData(length int, data []byte) string {
	if len(data) > length && !d.params.fullOutput {
		return fmt.Sprintf("%x...<%d bytes>", data[:length], len(data)-length)
	}
	return hex.EncodeToString(data)
}

func (d *DebugHandler) PrintReceipt(receipt *ReceiptInfo, indentEntry, indent string) {
	hasContract := receipt.Contract != nil

	makeKey := func(key string) string {
		key = keyColor.Sprint(key)
		return fmt.Sprintf("%s%-20s: ", indent, key)
	}

	makeKeyEntry := func(key string) string {
		key = keyColor.Sprint(key)
		return fmt.Sprintf("%s%-20s: ", indentEntry, key)
	}

	flags := receipt.Transaction.Flags.String()
	if receipt.Transaction.RequestId != 0 && !receipt.Transaction.Flags.IsResponse() {
		flags += ", Request"
	}

	fmt.Printf("%s0x%x\n", makeKeyEntry("Transaction"), receipt.Transaction.Hash)
	if hasContract {
		fmt.Printf("%s%s\n", makeKey("Contract"), color.MagentaString(receipt.Contract.ShortName()))
	}
	fmt.Printf("%s%s\n", makeKey("Flags"), color.YellowString(flags))
	fmt.Printf("%s%s\n", makeKey("Address"), receipt.Receipt.ContractAddress.Hex())
	if hasContract && !receipt.Transaction.Flags.GetBit(types.TransactionFlagResponse) {
		calldata, err := receipt.Contract.DecodeCallData(receipt.Transaction.Data)
		if err != nil {
			errStr := color.RedString("Failed to decode: %s", err.Error())
			fmt.Printf("%s[%s]%s\n", makeKey("CallData"), errStr, d.truncateData(48, receipt.Transaction.Data))
		} else {
			fmt.Printf("%s%s\n", makeKey("CallData"), calldataColor.Sprint(calldata))
		}
	} else if len(receipt.Transaction.Data) != 0 {
		fmt.Printf("%s%s\n", makeKey("CallData"), d.truncateData(96, receipt.Transaction.Data))
	}
	if len(receipt.Receipt.Logs) != 0 {
		fmt.Println(makeKey("Logs"))

		for i, log := range receipt.Receipt.Logs {
			if hasContract {
				if i == len(receipt.Receipt.Logs)-1 {
					indentEntry = indent + "\u2514 " // `└` symbol
				} else {
					indentEntry = indent + "\u251c " // `├` symbol
				}
				fmt.Print(indentEntry)

				decoded, err := receipt.Contract.DecodeLog(log)
				if err != nil || decoded == "" {
					fmt.Print("[", color.RedString("Failed to decode: %s", err.Error()), "]")
					logsJson, err := json.Marshal(log)
					if err == nil {
						fmt.Print(string(logsJson))
					}
				} else {
					fmt.Print(logsColor.Sprint(decoded))
				}
				fmt.Println()
			} else {
				logsJson, err := json.Marshal(log)
				if err == nil {
					fmt.Println(string(logsJson))
				}
			}
		}
	}
	if len(receipt.Receipt.DebugLogs) != 0 {
		fmt.Println(makeKey("Debug logs"))

		for i, log := range receipt.Receipt.DebugLogs {
			if i == len(receipt.Receipt.DebugLogs)-1 {
				indentEntry = indent + "\u2514 " // `└` symbol
			} else {
				indentEntry = indent + "\u251c " // `├` symbol
			}
			fmt.Print(indentEntry)

			fmt.Print(log.Message)
			if len(log.Message) > 0 {
				fmt.Print(": ")
				fmt.Print(log.Data)
			}
			fmt.Println()
		}
	}
	if !receipt.Receipt.Success {
		fmt.Printf("%s%s\n", makeKey("Status"), color.RedString(receipt.Receipt.Status))
		fmt.Printf("%s%d\n", makeKey("FailedPc"), receipt.Receipt.FailedPc)
	} else {
		fmt.Printf("%s%s\n", makeKey("Status"), color.GreenString(receipt.Receipt.Status))
	}
	if !receipt.Transaction.Flags.IsRefund() && !receipt.Transaction.Flags.IsBounce() {
		fmt.Printf("%s%d\n", makeKey("GasUsed"), receipt.Receipt.GasUsed)
		fmt.Printf("%s%s\n", makeKey("FeeCredit"), receipt.Transaction.FeeCredit)
		fmt.Printf("%s%s\n", makeKey("MaxFee"), receipt.Transaction.MaxFeePerGas)
		if !receipt.Transaction.MaxPriorityFeePerGas.IsZero() {
			fmt.Printf("%s%s\n", makeKey("PriorityFee"), receipt.Transaction.MaxPriorityFeePerGas)
		}
		fmt.Printf("%s%s\n", makeKey("GasPrice"), receipt.Receipt.GasPrice)
	}
	if !receipt.Transaction.Value.IsZero() {
		fmt.Printf("%s%s\n", makeKey("Value"), receipt.Transaction.Value)
	}
	if receipt.Transaction.RequestId != 0 {
		fmt.Printf("%s%d\n", makeKey("RequestId"), receipt.Transaction.RequestId)
	}
	fmt.Printf(
		"%s%d:%d\n", makeKey("Block"), receipt.Receipt.ContractAddress.ShardId(), receipt.Transaction.BlockNumber)

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

func (d *DebugHandler) PrintTransactionChain() {
	d.PrintReceipt(d.RootReceipt, "", "")
}

func runCommand(cmd *cobra.Command, args []string, params *debugParams) error {
	service := cliservice.NewService(cmd.Context(), common.GetRpcClient(), nil, nil)

	hashStr := args[0]

	var txnHash libcommon.Hash
	if err := txnHash.Set(hashStr); err != nil {
		return err
	}
	if txnHash == libcommon.EmptyHash {
		return errors.New("empty txnHash")
	}

	cometa := common.GetCometaRpcClient()

	debugHandler := NewDebugHandler(params, service, cometa, txnHash)

	receipt, err := service.FetchReceiptByHash(txnHash)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to fetch the receipt")
		return err
	}
	if receipt == nil {
		return errors.New("no receipt found for the transaction")
	}

	if err = debugHandler.CollectReceipts(receipt); err != nil {
		logger.Error().Err(err).Msg("Failed to collect the receipts")
		return err
	}

	debugHandler.PrintTransactionChain()

	fmt.Println()

	debugHandler.ShowFailures()

	return nil
}
