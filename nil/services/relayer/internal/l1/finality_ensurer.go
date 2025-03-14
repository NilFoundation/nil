package l1

import (
	"context"
	"errors"
)

type FinalityEnsurer struct {
	// TODO poll storage from listener emits & by ticker
	// fetch latest finalized block by ticker
	// for each block which number <= last finalized block number:
	//   - if it is included in chain - put it into the L2 event storage
	//   - if it is not in chain - ignore & add log
	//   - drop record from L1 evnet storage
	//   - emit event
	emitter chan struct{}
}

func (fe *FinalityEnsurer) Name() string {
	return "l1-block-finality-ensurer"
}

func (fe *FinalityEnsurer) Run(ctx context.Context, started chan<- struct{}) error {
	return errors.New("not implemented")
}

func (fe *FinalityEnsurer) EventFinalized() <-chan struct{} {
	return fe.emitter
}
