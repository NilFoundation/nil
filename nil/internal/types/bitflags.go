package types

import (
	"unsafe"

	"golang.org/x/exp/constraints"
)

type BitFlags[T constraints.Integer] struct {
	Bits T
}

func NewBitFlags[T constraints.Integer](flags ...int) BitFlags[T] {
	var b BitFlags[T]
	for _, i := range flags {
		b.SetBit(i)
	}
	return b
}

func (b BitFlags[T]) Set(i int, v bool) BitFlags[T] {
	if v {
		b.SetBit(i)
	} else {
		b.ClearBit(i)
	}
	return b
}

func (b *BitFlags[T]) SetBit(i int) {
	if uintptr(i) >= (unsafe.Sizeof(b.Bits) * 8) {
		panic("index out of range")
	}
	b.Bits |= 1 << i
}

func (b *BitFlags[T]) ClearBit(i int) {
	if uintptr(i) >= (unsafe.Sizeof(b.Bits) * 8) {
		panic("index out of range")
	}
	b.Bits &= ^(1 << i)
}

func (b *BitFlags[T]) GetBit(i int) bool {
	if uintptr(i) >= (unsafe.Sizeof(b.Bits) * 8) {
		panic("index out of range")
	}
	return b.Bits&(1<<i) != 0
}

func (b *BitFlags[T]) Clear() {
	b.Bits = 0
}

func (b *BitFlags[T]) BitsNum() int {
	return int(unsafe.Sizeof(b.Bits) * 8)
}
