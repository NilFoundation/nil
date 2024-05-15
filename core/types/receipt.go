package types

type Receipts []*Receipt

type Receipt struct {
	Success bool
	GasUsed uint32
	Bloom   Bloom
	Logs    []*Log `ssz-max:"1000"`
}

func (r *Receipt) AddLog(log *Log) {
	r.Logs = append(r.Logs, log)
}
