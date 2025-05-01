package executor

import (
	"context"
	"sync"

	"github.com/NilFoundation/nil/nil/services/synccommittee/internal/types"
)

type inMemoryIdSource struct {
	cachedId types.TaskExecutorId
	once     sync.Once
}

// NewInMemoryIdSource creates a new `IdSource` which generates a random `TaskExecutorId` on
// the first call of `GetCurrentId` and caches it in memory.
func NewInMemoryIdSource() IdSource {
	return &inMemoryIdSource{}
}

func (n *inMemoryIdSource) GetCurrentId(context.Context) (*types.TaskExecutorId, error) {
	n.once.Do(func() {
		n.cachedId = types.NewRandomExecutorId()
	})

	return &n.cachedId, nil
}

type IdStorage interface {
	GetOrAdd(ctx context.Context, idGenerator func() types.TaskExecutorId) (*types.TaskExecutorId, error)
}

type persistentIdSource struct {
	storage IdStorage
}

// NewPersistentIdSource creates a new `IdSource` backed by persistent storage
// to preserve the same `TaskExecutorId` value between application restarts.
func NewPersistentIdSource(storage IdStorage) IdSource {
	return &persistentIdSource{
		storage: storage,
	}
}

func (p *persistentIdSource) GetCurrentId(ctx context.Context) (*types.TaskExecutorId, error) {
	return p.storage.GetOrAdd(ctx, types.NewRandomExecutorId)
}
