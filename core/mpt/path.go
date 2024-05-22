package mpt

import (
	ssz "github.com/ferranbt/fastssz"
)

const (
	OddFlag  = 0x10
	LeafFlag = 0x20
)

type Path struct {
	data   []byte
	offset int
	IsLeaf bool
}

var (
	_ ssz.Marshaler   = new(Path)
	_ ssz.Unmarshaler = new(Path)
	_ ssz.HashRoot    = new(Path)
)

type PathAccessor interface {
	At(idx int) int
}

func newPath(data []byte, offset int) *Path {
	return &Path{data, offset, false}
}

func (path *Path) Size() int {
	return len(path.data)*2 - path.offset
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
	idx += path.offset
	nibbleIdx := idx % 2

	b := int(path.data[idx/2])

	if nibbleIdx == 0 {
		return b >> 4
	}

	return b & 0x0F
}

func (path *Path) Consume(amount int) *Path {
	path.offset += amount
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

	var offset int
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

func (path *Path) Encode() []byte {
	output := make([]byte, 0, path.Size())

	nibblesLen := path.Size()
	isOdd := nibblesLen%2 == 1

	prefix := byte(0x00)
	if isOdd {
		prefix += byte(OddFlag + path.At(0))
	}
	if path.IsLeaf {
		prefix += LeafFlag
	}

	output = append(output, prefix)

	pos := nibblesLen % 2

	for pos < nibblesLen {
		b := path.At(pos)*16 + path.At(pos+1)
		output = append(output, byte(b))
		pos += 2
	}

	return output
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

// MarshalSSZ ssz marshals the Path object
func (p *Path) MarshalSSZ() ([]byte, error) {
	return ssz.MarshalSSZ(p)
}

// MarshalSSZTo ssz marshals the Path object to a target array
func (p *Path) MarshalSSZTo(buf []byte) ([]byte, error) {
	return append(buf, p.Encode()...), nil
}

// UnmarshalSSZ ssz unmarshals the Path object
func (p *Path) UnmarshalSSZ(buf []byte) error {
	isOddLen := (buf[0] & OddFlag) == OddFlag
	if isOddLen {
		p.offset = 1
	} else {
		p.offset = 2
	}
	p.data = make([]byte, len(buf))
	copy(p.data, buf)
	return nil
}

// SizeSSZ returns the ssz encoded size in bytes for the Path object
func (p *Path) SizeSSZ() int {
	return 1 + p.Size()/2
}

// HashTreeRoot ssz hashes the Path object
func (p *Path) HashTreeRoot() ([32]byte, error) {
	return ssz.HashWithDefaultHasher(p)
}

// HashTreeRootWith ssz hashes the Path object with a hasher
func (p *Path) HashTreeRootWith(hh ssz.HashWalker) (err error) {
	indx := hh.Index()
	hh.AppendBytes32(p.Encode())
	hh.Merkleize(indx)
	return
}

// GetTree ssz hashes the Path object
func (p *Path) GetTree() (*ssz.Node, error) {
	return ssz.ProofTree(p)
}
