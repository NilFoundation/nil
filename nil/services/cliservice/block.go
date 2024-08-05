package cliservice

import (
	"encoding/json"
	"errors"
	"text/template"

	"github.com/NilFoundation/nil/nil/common"
	"github.com/NilFoundation/nil/nil/common/hexutil"
	"github.com/NilFoundation/nil/nil/internal/types"
)

// FetchBlock fetches the block by number or hash
func (s *Service) FetchBlock(shardId types.ShardId, blockId any) ([]byte, error) {
	blockData, err := s.client.GetBlock(shardId, blockId, true)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to fetch block")
		return nil, err
	}

	// Marshal the block data into a pretty-printed JSON format
	blockDataJSON, err := json.MarshalIndent(blockData, "", "  ")
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to marshal block data to JSON")
		return nil, err
	}

	s.logger.Info().Msgf("Fetched block:\n%s", blockDataJSON)
	return blockDataJSON, nil
}

// FetchDebugBlock fetches the block by number or hash with messages related data.
func (s *Service) FetchDebugBlock(shardId types.ShardId, blockId any, jsonOutput bool, fullOutput bool, noColor bool) ([]byte, error) {
	hexedBlock, err := s.client.GetDebugBlock(shardId, blockId, true)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to fetch block")
		return nil, err
	}

	block, err := hexedBlock.DecodeHexAndSSZ()
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to decode block data from hexed SSZ")
		return nil, err
	}

	if jsonOutput {
		return s.debugBlockToJson(shardId, block)
	} else {
		return s.debugBlockToText(shardId, block, !noColor, fullOutput)
	}
}

// We cannot make it generic because of https://stackoverflow.com/questions/78250015/go-embedded-type-cannot-be-a-type-parameter
type messageWithHash struct {
	types.Message
}

func (m *messageWithHash) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		*types.Message
		Hash common.Hash `json:"hash"`
	}{&m.Message, m.Message.Hash()})
}

func (s *Service) debugBlockToJson(shardId types.ShardId, block *types.BlockWithExtractedData) ([]byte, error) {
	toWithHashMessages := func(messages []*types.Message) []messageWithHash {
		result := make([]messageWithHash, 0, len(messages))
		for _, message := range messages {
			result = append(result, messageWithHash{*message})
		}
		return result
	}
	// Unfortunately, we have to make a copy of the messages in order to add hashes to them.
	// Because of this, we are duplicating the BlockWithExtractedData structure and if we extend it,
	// we will also need to support it here.
	// On the other hand, perhaps we want to control the output format more carefully, in which case it's not so bad.
	blockDataJSON, err := json.MarshalIndent(struct {
		*types.Block
		InMessages  []messageWithHash      `json:"inMessages"`
		OutMessages []messageWithHash      `json:"outMessages"`
		Receipts    []*types.Receipt       `json:"receipts"`
		Errors      map[common.Hash]string `json:"errors,omitempty"`
		Hash        common.Hash            `json:"hash"`
		ShardId     types.ShardId          `json:"shardId"`
	}{
		block.Block,
		toWithHashMessages(block.InMessages),
		toWithHashMessages(block.OutMessages),
		block.Receipts,
		block.Errors,
		block.Block.Hash(),
		shardId,
	}, "", "  ")
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to marshal block data to JSON")
		return nil, err
	}
	return blockDataJSON, nil
}

func (s *Service) debugBlockToText(shardId types.ShardId, block *types.BlockWithExtractedData, useColor bool, full bool) ([]byte, error) {
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
Block #{{ .block.Id }} [{{ .color.bold }}{{ .block.Hash }}{{ .color.reset }}] @ {{ .shardId }} shard
  PrevBlock: {{ .block.PrevBlock }}
  ChildBlocksRootHash: {{ .block.ChildBlocksRootHash }}
  MainChainHash: {{ .block.MainChainHash }}
{{ if len .block.InMessages -}}
▼ InMessages [{{ .block.InMessagesRoot }}]:
  {{- range $index, $element := .block.InMessages -}}
    {{ template "message" dict "message" $element "index" $index "color" $color "block" $block }}
  {{- end }}
{{- else -}}
■ No in messages [{{ .block.InMessagesRoot }}]
{{- end }}
{{ if len .block.OutMessages -}}
▼ OutMessages [{{ .block.OutMessagesRoot }}]:
  {{- range $index, $element := .block.OutMessages -}}
    {{ template "message" dict "message" $element "index" $index "color" $color "block" $block }}
  {{- end }}
{{- else -}}
■ No out messages [{{ .block.OutMessagesRoot }}]
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
  {{- range $messageHash, $message := . }}
    {{ $messageHash }}: {{ $color.red }}{{ $message }}{{ $color.reset }}
  {{- end }}
{{- end -}}
`

	messageTemplate := `
  # {{ .index }} [{{ .color.bold }}{{.message.Hash}}{{ .color.reset }}] | {{ .color.blue }}{{ .message.From }}{{ .color.reset }} => {{ .color.magenta }}{{ .message.To }}{{ .color.reset }}
    {{- $color := .color }}
    {{- with findReceipt .block.Receipts .message.Hash }}
    {{ $color.yellow }}Status: {{ if .Success }}{{ $color.green }}{{ else }}{{ $color.red }}{{ end }}{{ .Status }}{{ $color.reset }}
    {{ $color.yellow }}GasUsed:{{ $color.reset }} {{ .GasUsed }}
    {{- end }}
    {{- with index .block.Errors .message.Hash }}
    {{ $color.yellow }}Error: {{ $color.red}}{{ . }}{{ $color.reset}}
	{{- end }}
    Flags: {{ .message.Flags }}
    RefundTo: {{ .message.RefundTo }}
    BounceTo: {{ .message.BounceTo }}
    Value: {{ .message.Value }}
    ChainId: {{ .message.ChainId }}
    Seqno: {{ .message.Seqno }}
    {{- with .message.Currency }}
  ▼ Currency:{{ range . }}
      {{ .Currency }}: {{ .Balance }}
	{{- end }}{{ end }}
    Data: {{ formatData .message.Data }}{{ with .message.Signature }}
    Signature: {{ . }}{{ end }}`

	receiptTemplate := `
  [{{ .color.bold }}{{ .receipt.MsgHash }}{{ .color.reset }}]
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
					if receipt.MsgHash == hash {
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
		},
		map[string]string{
			"message": messageTemplate,
			"receipt": receiptTemplate,
		})
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to parse block template")
		return []byte{}, err
	}

	return []byte(text), nil
}
