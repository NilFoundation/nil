package metrics

import (
	"github.com/NilFoundation/nil/nil/internal/telemetry"
)

var (
	Meter        = telemetry.NewMeter("stresser")
	PendingTxNum = telemetry.Int64UpDownCounter(Meter, "pending_tx_num")
	TotalTxNum   = telemetry.Int64Gauge(Meter, "total_tx_num")
	FailedTxNum  = telemetry.Int64Counter(Meter, "failed_tx_num")
	SuccessTxNum = telemetry.Int64Counter(Meter, "success_tx_num")
)
