package mpt

import (
	ssz "github.com/NilFoundation/fastssz"
)

type Path struct {
	Data   []byte `ssz-max:"100000"`
	Offset uint32
}

var (
	_ ssz.Marshaler   = new(Path)
	_ ssz.Unmarshaler = new(Path)
	_ ssz.HashRoot    = new(Path)
)

type PathAccessor interface {
	At(idx int) int
}

func newPath(data []byte, offset uint32) *Path {
	return &Path{data, offset}
}

func (path *Path) Size() int {
	return len(path.Data)*2 - int(path.Offset)
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
	idx += int(path.Offset)
	nibbleIdx := idx % 2

	b := int(path.Data[idx/2])

	if nibbleIdx == 0 {
		return b >> 4
	}

	return b & 0x0F
}

func (path *Path) Consume(amount int) *Path {
	path.Offset += uint32(amount)
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

	var offset uint32
	if isOddLen {
		offset = 1
	} else {
		offset = 0
	}

	return newPath(data, offset)
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
