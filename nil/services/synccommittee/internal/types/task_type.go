package types

import (
	"fmt"
	"maps"
	"slices"
)

// TaskType Tasks have different types, it affects task input and priority
type TaskType uint8

const (
	TaskTypeNone TaskType = iota
	ProofBatch
	PartialProve
	AggregatedChallenge
	CombinedQ
	AggregatedFRI
	FRIConsistencyChecks
	MergeProof
)

var TaskTypes = map[string]TaskType{
	"ProofBatch":           ProofBatch,
	"PartialProve":         PartialProve,
	"AggregatedChallenge":  AggregatedChallenge,
	"CombinedQ":            CombinedQ,
	"AggregatedFRI":        AggregatedFRI,
	"FRIConsistencyChecks": FRIConsistencyChecks,
	"MergeProof":           MergeProof,
}

func (t *TaskType) Set(str string) error {
	if v, ok := TaskTypes[str]; ok {
		*t = v
		return nil
	}
	return fmt.Errorf("unknown task type: %s", str)
}

func (*TaskType) Type() string {
	return "TaskType"
}

func (*TaskType) PossibleValues() []string {
	return slices.Collect(maps.Keys(TaskTypes))
}
