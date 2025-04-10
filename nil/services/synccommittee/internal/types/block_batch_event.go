package types

import (
	"time"

	"github.com/google/uuid"
)

type BatchEventId uuid.UUID

func NewBatchEventId() BatchEventId    { return BatchEventId(uuid.New()) }
func (id BatchEventId) String() string { return uuid.UUID(id).String() }
func (id BatchEventId) Bytes() []byte  { return []byte(id.String()) }

type BatchEvent struct {
	Id        BatchEventId `json:"id"`
	BatchId   BatchId      `json:"batchId"`
	NewStatus BatchStatus  `json:"newStatus"`
	CreatedAt time.Time    `json:"createdAt"`
}

func NewBatchEvent(batchId BatchId, newStatus BatchStatus, currentTime time.Time) BatchEvent {
	return BatchEvent{
		Id:        NewBatchEventId(),
		BatchId:   batchId,
		NewStatus: newStatus,
		CreatedAt: currentTime,
	}
}
