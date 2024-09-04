package mpt

import (
	ssz "github.com/NilFoundation/fastssz"
	"github.com/NilFoundation/nil/nil/common/check"
)

type Path struct {
	Data []byte `ssz-max:"100000"`
	// whether path starts from 1st nibble
	Shifted bool
	// we can't store FirstNibble as part of Data
	// https://github.com/NilFoundation/nil/pull/836#discussion_r1743673490
	FirstNibble uint8
}

var (
	_ ssz.Marshaler   = new(Path)
	_ ssz.Unmarshaler = new(Path)
	_ ssz.HashRoot    = new(Path)
)

type PathAccessor interface {
	At(idx int) int
}

func newPath(data []byte, shifted bool) *Path {
	if shifted {
		check.PanicIfNot(len(data) > 0)
		return &Path{Data: data[1:], Shifted: true, FirstNibble: data[0] & 0x0F} //nolint:gosec
	}
	return &Path{Data: data}
}

func (path *Path) Size() int {
	size := 2 * len(path.Data)
	if path.Shifted {
		size++
	}
	return size
}

func (path *Path) Empty() bool {
	return path.Size() == 0
}

func (path *Path) Equal(other *Path) bool {
	if other.Size() != path.Size() {
		return false
	}

	for i := range path.Size() {
		if path.At(i) != other.At(i) {
			return false
		}
	}

	return true
}

func (path *Path) StartsWith(other *Path) bool {
	if other.Size() > path.Size() {
		return false
	}

	for i := range other.Size() {
		if path.At(i) != other.At(i) {
			return false
		}
	}

	return true
}

func (path *Path) At(idx int) int {
	if path.Shifted {
		// first nibble is stored in a separate field
		if idx == 0 {
			return int(path.FirstNibble)
		}
		idx--
	}

	nibbleIdx := idx % 2

	b := int(path.Data[idx/2])

	if nibbleIdx == 0 {
		return b >> 4
	}

	return b & 0x0F
}

func (path *Path) Consume(offset int) *Path {
	if offset == 0 {
		return path
	}
	if path.Shifted {
		// first nibble is stored in a separate field
		offset--
	}

	path.Data = path.Data[offset/2:]

	if offset%2 == 1 {
		path.Shifted = true
		path.FirstNibble = path.Data[0] & 0x0F
		path.Data = path.Data[1:]
	} else {
		path.Shifted = false
		path.FirstNibble = 0
	}
	return path
}

func createNew[T PathAccessor](path T, length int) *Path {
	data := make([]byte, 0, length)

	isOddLen := length%2 == 1
	pos := 0

	if isOddLen {
		data = append(data, byte(path.At(pos)))
		pos += 1
	}

	for pos < length {
		data = append(data, byte(path.At(pos)*16+path.At(pos+1)))
		pos += 2
	}

	return newPath(data, isOddLen)
}

func (path *Path) CommonPrefix(other *Path) *Path {
	leastLen := min(path.Size(), other.Size())
	commonLen := 0
	for i := range leastLen {
		if path.At(i) != other.At(i) {
			break
		}
		commonLen += 1
	}
	return createNew(path, commonLen)
}

type Chained struct {
	first  *Path
	second *Path
}

func (c *Chained) At(idx int) int {
	if idx < c.first.Size() {
		return c.first.At(idx)
	} else {
		return c.second.At(idx - c.first.Size())
	}
}

func (c *Chained) Size() int {
	return c.first.Size() + c.second.Size()
}

func (path *Path) Combine(other *Path) *Path {
	c := Chained{path, other}
	return createNew(&c, c.Size())
}

func CommonPrefix(paths []*Path) *Path {
	if len(paths) == 0 {
		return nil
	}

	prefix := paths[0]
	for _, p := range paths[1:] {
		prefix = prefix.CommonPrefix(p)
		if prefix.Size() == 0 {
			break
		}
	}

	return prefix
}
