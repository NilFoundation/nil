package storage

import (
	"context"

	"github.com/NilFoundation/nil/nil/internal/db"
	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/internal/telemetry/telattr"
	"github.com/NilFoundation/nil/nil/services/relayer/internal/metrics"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type TableMetrics interface {
	RecordInserts(context.Context, db.TableName, int)
	RecordDeletes(context.Context, db.TableName, int)
	SetTableSize(context.Context, db.TableName, int)
}

const (
	opLabel = "operation"

	rwOpDel    = "delete"
	rwOpInsert = "insert"
)

type tableMetrics struct {
	attrs     metric.MeasurementOption
	sizeGauge telemetry.Gauge
	opCounter telemetry.Counter
}

func NewTableMetrics() (TableMetrics, error) {
	tm := &tableMetrics{}
	if err := metrics.InitMetrics(tm, "relayer", "storage"); err != nil {
		return nil, err
	}
	return tm, nil
}

func (tm *tableMetrics) Init(name string, meter telemetry.Meter, attrs metric.MeasurementOption) error {
	var err error

	tm.sizeGauge, err = meter.Int64Gauge(name + ".table_size")
	if err != nil {
		return err
	}

	tm.opCounter, err = meter.Int64Counter(name + ".table_operations")
	if err != nil {
		return err
	}

	tm.attrs = attrs

	return nil
}

func (tm *tableMetrics) RecordInserts(ctx context.Context, table db.TableName, val int) {
	tm.opCounter.Add(
		ctx,
		int64(val),
		tm.attrs,
		telattr.With(
			attribute.String(opLabel, rwOpInsert),
			attribute.String("table_name", string(table)),
		),
	)
}

func (tm *tableMetrics) RecordDeletes(ctx context.Context, table db.TableName, val int) {
	tm.opCounter.Add(
		ctx,
		int64(val),
		tm.attrs,
		telattr.With(
			attribute.String(opLabel, rwOpDel),
			attribute.String("table_name", string(table)),
		),
	)
}

func (tm *tableMetrics) SetTableSize(ctx context.Context, table db.TableName, size int) {
	tm.sizeGauge.Record(
		ctx,
		int64(size),
		tm.attrs,
		telattr.With(
			attribute.String("table_name", string(table)),
		),
	)
}
