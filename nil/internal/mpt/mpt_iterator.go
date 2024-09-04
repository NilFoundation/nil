package mpt

import (
	"iter"
)

func (m *Reader) Iterate() iter.Seq2[[]byte, []byte] {
	type Yield = func([]byte, []byte) bool
	return func(yield Yield) {
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
				// note: even though we access path.Data directly here is ok
				// cause every key in the mpt is []byte, i.e. it consists of even number of nibbles
				if !yield(path.Data, data) {
					return
				}
			}
			switch node := node.(type) {
			case *BranchNode:
				for i, br := range node.Branches {
					if len(br) > 0 {
						iter(br, path.Combine(newPath([]byte{byte(i)}, true)))
					}
				}
				return
			case *ExtensionNode:
				iter(node.NextRef, path)
			}
		}
		if m.root.IsValid() {
			iter(m.root, newPath(nil, false))
		}
	}
}
