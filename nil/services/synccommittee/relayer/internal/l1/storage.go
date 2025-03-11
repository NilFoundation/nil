package l1

import (
	"context"
	"errors"
)

type EventStorage struct {
	// BadgerDB goes here
	// TODO store events from L1
	// TODO store last processed block number
	// TODO use sequencer for event ordering
}

func (es *EventStorage) StoreEvent(ctx context.Context, evt *Event) error {
	// TODO add sequence number
	return errors.New("not implemented")
}

func (es *EventStorage) GetEvents(ctx context.Context) ([]*Event, error) {
	return nil, errors.New("not implemented")
}

func (es *EventStorage) DeleteEvents(ctx context.Context, ids []EventID) error {
	return errors.New("not implemented")
}

func (es *EventStorage) GetLastProcessedBlock(ctx context.Context) (*ProcessedBlock, error) {
	return nil, errors.New("not implemented")
}

func (es *EventStorage) SetLastProcessedBlock(ctx context.Context, blk *ProcessedBlock) error {
	return errors.New("not implemented")
}
