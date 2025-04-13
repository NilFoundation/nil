// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package vm

import (
	"errors"
	"sync"

	"github.com/NilFoundation/nil/nil/common/check"
	"github.com/holiman/uint256"
)

var stackPool = sync.Pool{
	New: func() any {
		return &Stack{data: make([]uint256.Int, 0, 16)}
	},
}

// Stack is an object for basic stack operations. Items popped to the stack are
// expected to be changed and modified. Stack does not take care of adding newly
// initialized objects.
type Stack struct {
	data []uint256.Int
}

func newStack() *Stack {
	res, ok := stackPool.Get().(*Stack)
	check.PanicIfNot(ok)
	return res
}

func returnStack(s *Stack) {
	s.data = s.data[:0]
	stackPool.Put(s)
}

// Data returns the underlying uint256.Int array.
func (st *Stack) Data() []uint256.Int {
	return st.data
}

func (st *Stack) push(d *uint256.Int) {
	// NOTE push limit (1024) is checked in baseCheck
	st.data = append(st.data, *d)
}

func (st *Stack) pop() (ret uint256.Int) {
	ret = st.data[len(st.data)-1]
	st.data = st.data[:len(st.data)-1]
	return
}

func (st *Stack) len() int {
	return len(st.data)
}

func (st *Stack) swap(n int) {
	st.data[st.len()-n], st.data[st.len()-1] = st.data[st.len()-1], st.data[st.len()-n]
}

func (st *Stack) dup(n int) {
	st.push(&st.data[st.len()-n])
}

func (st *Stack) peek() *uint256.Int {
	return &st.data[st.len()-1]
}

// Back returns the n'th item in stack
func (st *Stack) Back(n int) *uint256.Int {
	return &st.data[st.len()-n-1]
}

func (st *Stack) CopyToBytes() []byte {
	res := make([]byte, 0, len(st.data)*32)
	for i := range len(st.data) {
		data := st.data[i].Bytes32()
		res = append(res, data[:]...)
	}
	return res
}

func (st *Stack) FromBytes(data []byte) error {
	if len(data)%32 != 0 {
		return errors.New("stack data length is not a multiple of 32")
	}
	st.data = make([]uint256.Int, 0, len(data)/32)
	for i := 0; i < len(data); i += 32 {
		st.data = append(st.data, *uint256.NewInt(0).SetBytes(data[i : i+32]))
	}
	return nil
}
