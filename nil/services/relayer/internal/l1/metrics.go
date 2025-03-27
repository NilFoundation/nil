package l1

import (
	"context"

	"github.com/NilFoundation/nil/nil/internal/telemetry"
	"github.com/NilFoundation/nil/nil/internal/telemetry/telattr"
	"github.com/NilFoundation/nil/nil/services/relayer/internal/metrics"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type EventListenerMetrics interface {
	SetFetcherActive(ctx context.Context)
	SetFetcherIdle(ctx context.Context)
	AddEventFromFetcher(ctx context.Context)
	AddEventFromSubscriber(ctx context.Context)
	AddSubscriptionError(ctx context.Context)
}

const (
	eventSourceLabel      = "event_source"
	eventSourceFetcher    = "fetcher"
	eventSourceSubscriber = "subscriber"

	eventStatusLabel     = "event_status"
	eventStatusFinalized = "finalized"
	eventStatusOrphaned  = "orphaned"
)

type eventListenerMetrics struct {
	attrs metric.MeasurementOption

	fetcherRunStatus telemetry.Gauge // 0 if fetcher is inactive
	subsciptionError telemetry.Counter
	eventsProcessed  telemetry.Counter
}

func NewEventListenerMetrics() (EventListenerMetrics, error) {
	elm := &eventListenerMetrics{}
	if err := metrics.InitMetrics(elm, "relayer", "event_listener"); err != nil {
		return nil, err
	}
	return elm, nil
}

func (elm *eventListenerMetrics) Init(name string, meter telemetry.Meter, attrs metric.MeasurementOption) error {
	var err error

	elm.fetcherRunStatus, err = meter.Int64Gauge(name + ".fetcher_active")
	if err != nil {
		return err
	}

	elm.subsciptionError, err = meter.Int64Counter(name + ".subscription_failure")
	if err != nil {
		return err
	}

	elm.eventsProcessed, err = meter.Int64Counter(name + ".events_processed")
	if err != nil {
		return err
	}

	elm.attrs = attrs
	return nil
}

func (elm *eventListenerMetrics) SetFetcherActive(ctx context.Context) {
	elm.fetcherRunStatus.Record(ctx, 1, elm.attrs)
}

func (elm *eventListenerMetrics) SetFetcherIdle(ctx context.Context) {
	elm.fetcherRunStatus.Record(ctx, 0, elm.attrs)
}

func (elm *eventListenerMetrics) AddEventFromFetcher(ctx context.Context) {
	sourceAttr := telattr.With(attribute.String(eventSourceLabel, eventSourceFetcher))
	elm.eventsProcessed.Add(ctx, 1, elm.attrs, sourceAttr)
}

func (elm *eventListenerMetrics) AddEventFromSubscriber(ctx context.Context) {
	sourceAttr := telattr.With(attribute.String(eventSourceLabel, eventSourceSubscriber))
	elm.eventsProcessed.Add(ctx, 1, elm.attrs, sourceAttr)
}

func (elm *eventListenerMetrics) AddSubscriptionError(ctx context.Context) {
	elm.subsciptionError.Add(ctx, 1, elm.attrs)
}

type FinalityEnsurerMetrics interface {
	SetTimeSinceFinalizedBlockNumberUpdate(ctx context.Context, sec uint64)
	AddRelayError(ctx context.Context)
	AddFinalizedEvents(ctx context.Context, count uint64)
	AddOrphanedEvents(ctx context.Context, count uint64)
}

type finalityEnsurerMetrics struct {
	attrs metric.MeasurementOption

	finalizedBlockUpdateLag telemetry.Gauge
	relayErrors             telemetry.Counter
	processedEvents         telemetry.Counter
}

func NewFinalityEnsurerMetrics() (FinalityEnsurerMetrics, error) {
	fem := &finalityEnsurerMetrics{}
	if err := metrics.InitMetrics(fem, "relayer", "finality_ensurer"); err != nil {
		return nil, err
	}
	return fem, nil
}

func (fem *finalityEnsurerMetrics) Init(name string, meter telemetry.Meter, attrs metric.MeasurementOption) error {
	var err error

	fem.finalizedBlockUpdateLag, err = meter.Int64Gauge(name + ".finalized_block_update_lag_sec")
	if err != nil {
		return err
	}

	fem.relayErrors, err = meter.Int64Counter(name + ".relay_error")
	if err != nil {
		return err
	}

	fem.processedEvents, err = meter.Int64Counter(name + ".processed_events")
	if err != nil {
		return err
	}

	fem.attrs = attrs
	return nil
}

func (fem *finalityEnsurerMetrics) SetTimeSinceFinalizedBlockNumberUpdate(ctx context.Context, sec uint64) {
	fem.finalizedBlockUpdateLag.Record(ctx, int64(sec), fem.attrs)
}

func (fem *finalityEnsurerMetrics) AddRelayError(ctx context.Context) {
	fem.relayErrors.Add(ctx, 1, fem.attrs)
}

func (fem *finalityEnsurerMetrics) AddFinalizedEvents(ctx context.Context, count uint64) {
	fem.processedEvents.Add(ctx, int64(count),
		telattr.With(attribute.String(eventStatusLabel, eventStatusFinalized)),
		fem.attrs,
	)
}

func (fem *finalityEnsurerMetrics) AddOrphanedEvents(ctx context.Context, count uint64) {
	fem.processedEvents.Add(ctx, int64(count),
		telattr.With(attribute.String(eventStatusLabel, eventStatusOrphaned)),
		fem.attrs,
	)
}
