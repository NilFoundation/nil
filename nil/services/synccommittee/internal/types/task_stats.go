package types

import (
	"context"
	"log"
)

type TaskStatNumbers struct {
	ActiveCount  uint32
	PendingCount uint32
}

type TaskStats struct {
	CountPerType     map[TaskType]TaskStatNumbers
	CountPerExecutor map[TaskExecutorId]uint32
}

func NewEmptyTaskStats() *TaskStats {
	return &TaskStats{
		CountPerType:     make(map[TaskType]TaskStatNumbers),
		CountPerExecutor: make(map[TaskExecutorId]uint32),
	}
}

func (s *TaskStats) Add(entry *TaskEntry) {
	statNumbersByType := s.CountPerType[entry.Task.TaskType]
	switch entry.Status {
	case Running:
		statNumbersByType.ActiveCount++
		s.CountPerType[entry.Task.TaskType] = statNumbersByType
		s.CountPerExecutor[entry.Owner]++

	case WaitingForExecutor, WaitingForInput:
		statNumbersByType.PendingCount++
		s.CountPerType[entry.Task.TaskType] = statNumbersByType

	case Failed, Completed:
		return

	case TaskStatusNone:
		log.Panicf("task %s has undefined status %s", entry.Task.Id, entry.Status)
	}
}

type TaskStatsProvider interface {
	GetTaskStats(ctx context.Context) (*TaskStats, error)
}
