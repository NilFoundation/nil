package cliservice

import (
	"encoding/json"
	"errors"
	"text/template"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/types"
)

// FetchDebugBlock fetches the block by number or hash with transactions related data.
func (s *Service) FetchDebugBlock(
	shardId types.ShardId,
	blockId any,
	jsonOutput bool,
	fullOutput bool,
	noColor bool,
) ([]byte, error) {
	hexedBlock, err := s.client.GetDebugBlock(s.ctx, shardId, blockId, true)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to fetch block")
		return nil, err
	}

	if hexedBlock == nil {
		return nil, nil
	}

	block, err := hexedBlock.DecodeSSZ()
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to decode block data from hexed SSZ")
		return nil, err
	}

	if jsonOutput {
		return s.debugBlockToJson(shardId, block)
	}
	return s.debugBlockToText(shardId, block, !noColor, fullOutput)
}

// We cannot make it generic because of
// https://stackoverflow.com/questions/78250015/go-embedded-type-cannot-be-a-type-parameter
type transactionWithHash struct {
	types.Transaction
}

func (m *transactionWithHash) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		*types.Transaction
		Hash common.Hash `json:"hash"`
	}{&m.Transaction, m.Hash()})
}

func (s *Service) debugBlockToJson(shardId types.ShardId, block *types.BlockWithExtractedData) ([]byte, error) {
	toWithHashTransactions := func(transactions []*types.Transaction) []transactionWithHash {
		result := make([]transactionWithHash, 0, len(transactions))
		for _, transaction := range transactions {
			result = append(result, transactionWithHash{*transaction})
		}
		return result
	}
	// Unfortunately, we have to make a copy of the transactions in order to add hashes to them.
	// Because of this, we are duplicating the BlockWithExtractedData structure and if we extend it,
	// we will also need to support it here.
	// On the other hand, perhaps we want to control the output format more carefully, in which case it's not so bad.
	blockDataJSON, err := json.MarshalIndent(struct {
		*types.Block
		ChildBlocks     []common.Hash          `json:"childBlocks"`
		InTransactions  []transactionWithHash  `json:"inTransactions"`
		OutTransactions []transactionWithHash  `json:"outTransactions"`
		Receipts        []*types.Receipt       `json:"receipts"`
		Errors          map[common.Hash]string `json:"errors,omitempty"`
		Hash            common.Hash            `json:"hash"`
		ShardId         types.ShardId          `json:"shardId"`
		BaseFee         types.Value            `json:"baseFee"`
		GasUsed         types.Gas              `json:"gasUsed"`
	}{
		block.Block,
		block.ChildBlocks,
		toWithHashTransactions(block.InTransactions),
		toWithHashTransactions(block.OutTransactions),
		block.Receipts,
		block.Errors,
		block.Hash(shardId),
		shardId,
		block.BaseFee,
		block.GasUsed,
	}, "", "  ")
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to marshal block data to JSON")
		return nil, err
	}
	return blockDataJSON, nil
}

func (s *Service) debugBlockToText(
	shardId types.ShardId,
	block *types.BlockWithExtractedData,
	useColor bool,
	full bool,
) ([]byte, error) {
	colors := map[string]string{
		"blue":    "\033[94m",
		"green":   "\033[32m",
		"magenta": "\033[95m",
		"red":     "\033[31m",
		"yellow":  "\033[93m",
		"reset":   "\033[0m",
		"bold":    "\033[1m",
	}
	if !useColor {
		for k := range colors {
			colors[k] = ""
		}
	}

	blockTemplate := `
{{- $block := .block -}}
{{- $color := .color -}}
Block #{{ .block.Id }} [{{ .color.bold }}{{ .block.Hash .shardId }}{{ .color.reset }}] @ {{ .shardId }} shard
  PrevBlock: {{ .block.PrevBlock }}
  BaseFee: {{ .block.BaseFee }}
  GasUsed: {{ .block.GasUsed }}
  ChildBlocksRootHash: {{ .block.ChildBlocksRootHash }}
{{- if len .block.ChildBlocks}}
  ChildBlocks:
  {{- range $index, $element := .block.ChildBlocks }}
    - {{ inc $index }}: {{ $element }}
  {{- end }}
{{- end}}
  MainShardHash: {{ .block.MainShardHash }}
{{ if len .block.InTransactions -}}
▼ InTransactions [{{ .block.InTransactionsRoot }}]:
  {{- range $index, $element := .block.InTransactions -}}
    {{ template "transaction" dict "transaction" $element "index" $index "color" $color "block" $block }}
  {{- end }}
{{- else -}}
■ No in transactions [{{ .block.InTransactionsRoot }}]
{{- end }}
{{ if len .block.OutTransactions -}}
▼ OutTransactions [{{ .block.OutTransactionsRoot }}]:
  {{- range $index, $element := .block.OutTransactions -}}
    {{ template "transaction" dict "transaction" $element "index" $index "color" $color "block" $block }}
  {{- end }}
{{- else -}}
■ No out transactions [{{ .block.OutTransactionsRoot }}]
{{- end }}
{{ if len .block.Receipts -}}
▼ Receipts [{{ .block.ReceiptsRoot }}]:
  {{- range .block.Receipts }}
    {{- template "receipt" dict "receipt" . "color" $color }}
  {{- end }}
{{- else -}}
■ No receipts [{{ .block.ReceiptsRoot }}]
{{- end }}
{{ with .block.Errors -}}
{{ $color.red }}▼ Errors:{{ $color.reset }}
  {{- range $transactionHash, $transaction := . }}
    {{ $transactionHash }}: {{ $color.red }}{{ $transaction }}{{ $color.reset }}
  {{- end }}
{{- end -}}
`

	transactionTemplate := `
  # {{ .index }} [{{ .color.bold }}{{.transaction.Hash}}{{ .color.reset }}] | {{ .color.blue }}{{ .transaction.From }}{{ .color.reset }} => {{ .color.magenta }}{{ .transaction.To }}{{ .color.reset }}
    {{- $color := .color }}
    {{- with findReceipt .block.Receipts .transaction.Hash }}
    {{ $color.yellow }}Status: {{ if .Success }}{{ $color.green }}{{ else }}{{ $color.red }}{{ end }}{{ .Status }}{{ $color.reset }}
    {{ $color.yellow }}GasUsed:{{ $color.reset }} {{ .GasUsed }}
    {{- end }}
    {{- with index .block.Errors .transaction.Hash }}
    {{ $color.yellow }}Error: {{ $color.red}}{{ . }}{{ $color.reset}}
    {{- end }}
    Flags: {{ .transaction.Flags }}
    RefundTo: {{ .transaction.RefundTo }}
    BounceTo: {{ .transaction.BounceTo }}
    Value: {{ .transaction.Value }}
    ChainId: {{ .transaction.ChainId }}
    Seqno: {{ .transaction.Seqno }}
    {{- with .transaction.Token }}
  ▼ Token:{{ range . }}
      {{ .Token }}: {{ .Balance }}
    {{- end }}{{ end }}
    Data: {{ formatData .transaction.Data }}{{ with .transaction.Signature }}
    Signature: {{ . }}{{ end }}`

	receiptTemplate := `
  [{{ .color.bold }}{{ .receipt.TxnHash }}{{ .color.reset }}]
     Status:
       {{- if .receipt.Success }}{{ .color.green }}{{ else }}{{ .color.red }}{{ end }}
       {{- " " }}{{ .receipt.Status }}
       {{- .color.reset }}
     GasUsed: {{ .receipt.GasUsed }}
     {{- /* */ -}}
`

	text, err := common.ParseTemplates(
		blockTemplate,
		map[string]any{
			"block":   block,
			"shardId": shardId,
			"color":   colors,
		},
		template.FuncMap{
			"dict": func(values ...any) (map[string]any, error) {
				if len(values)%2 != 0 {
					return nil, errors.New("invalid dict call")
				}
				dict := make(map[string]any, len(values)/2)
				for i := 0; i < len(values); i += 2 {
					key, ok := values[i].(string)
					if !ok {
						return nil, errors.New("dict keys must be strings")
					}
					dict[key] = values[i+1]
				}
				return dict, nil
			},
			"findReceipt": func(receipts []*types.Receipt, hash common.Hash) *types.Receipt {
				for _, receipt := range receipts {
					if receipt.TxnHash == hash {
						return receipt
					}
				}
				return nil
			},
			"formatData": func(data []byte) string {
				if len(data) == 0 {
					return "<empty>"
				}
				hexed := hexutil.Encode(data)
				limit := 100
				if full || len(hexed) < limit {
					return hexed
				}
				return hexed[:limit] + "... (run with --full to expand)"
			},
			"inc": func(i int) int {
				return i + 1
			},
		},
		map[string]string{
			"transaction": transactionTemplate,
			"receipt":     receiptTemplate,
		})
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to parse block template")
		return []byte{}, err
	}

	return []byte(text), nil
}
