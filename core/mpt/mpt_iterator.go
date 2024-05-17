package mpt

type MptIteratorKey struct {
	Key   []byte
	Value []byte
}

// TODO: Fix leaked coroutine
func (m *MerklePatriciaTrie) Iterate() chan MptIteratorKey {
	out := make(chan MptIteratorKey)
	go func() {
		defer close(out)
		var iter func(ref Reference, path *Path)
		iter = func(ref Reference, path *Path) {
			node, err := m.getNode(ref)
			if err != nil {
				return
			}
			npath := node.Path()
			if npath != nil {
				path = path.Combine(npath)
			}
			data := node.Data()
			if len(data) > 0 {
				out <- MptIteratorKey{Key: path.data, Value: data}
			}
			switch node := node.(type) {
			case *BranchNode:
				for i, br := range node.Branches {
					if len(br) > 0 {
						iter(br, path.Combine(newPath([]byte{byte(i)}, 1)))
					}
				}
				return
			case *ExtensionNode:
				iter(node.NextRef, path)
			}
		}
		iter(m.root, newPath(nil, 0))
	}()
	return out
}
