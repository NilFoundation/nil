package l1

import (
	"context"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/event"
)

type L1Contract interface {
	SubscribeToEvents(ctx context.Context, sink chan<- *L1MessageSent) (event.Subscription, error)
	GetEventsFromBlockRange(ctx context.Context, from uint64, to *uint64) ([]*L1MessageSent, error)
}

type l1ContractWrapper struct {
	impl   *L1
	l2Addr common.Address
}

var _ L1Contract = (*l1ContractWrapper)(nil)

func NewL1ContractWrapper(ethClient EthClient,
	l1ContractAddr, l2ConractAddr string,
) (*l1ContractWrapper, error) {
	addr := common.HexToAddress(l1ContractAddr)
	impl, err := NewL1(addr, ethClient)
	if err != nil {
		return nil, err
	}

	return &l1ContractWrapper{
		impl:   impl,
		l2Addr: common.HexToAddress(l2ConractAddr),
	}, nil
}

func (w *l1ContractWrapper) SubscribeToEvents(
	ctx context.Context,
	sink chan<- *L1MessageSent,
) (event.Subscription, error) {
	return w.impl.WatchMessageSent(
		&bind.WatchOpts{Context: ctx},
		sink,
		nil,                        // any sender (for now)
		[]common.Address{w.l2Addr}, // destination is the contract this relayer is bound to
		nil,                        // any nonce
	)
}

func (w *l1ContractWrapper) GetEventsFromBlockRange(
	ctx context.Context,
	from uint64,
	to *uint64,
) ([]*L1MessageSent, error) {
	iter, err := w.impl.FilterMessageSent(
		&bind.FilterOpts{
			Start: from,
			End:   to,
		},
		nil,                        // any sender (for now)
		[]common.Address{w.l2Addr}, // destination is the contract this relayer is bound to
		nil,                        // any nonce
	)
	if err != nil {
		return nil, err
	}

	// oclaw: it is not expected to be too much events here, so getting rid of iterator for simplicity
	// can be changed though
	var ret []*L1MessageSent
	for iter.Next() {
		ret = append(ret, iter.Event)
	}

	return ret, iter.Error()
}
