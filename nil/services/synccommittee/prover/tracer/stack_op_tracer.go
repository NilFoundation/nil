package tracer

import (
	"fmt"

	"github.com/NilFoundation/nil/nil/internal/tracing"
	"github.com/NilFoundation/nil/nil/internal/types"
	"github.com/NilFoundation/nil/nil/internal/vm"
	"github.com/holiman/uint256"
)

type StackCopy []uint256.Int

type StackOp struct {
	IsRead bool // Is write otherwise
	Idx    int  // Index of element in stack
	Value  types.Uint256
	PC     uint64
	TxnId  uint
	RwIdx  uint
}

type StackOpTracer struct {
	res   []StackOp
	rwCtr *RwCounter
	txnId uint

	opCode         vm.OpCode
	pc             uint64
	scope          tracing.OpContext
	prevOpFinisher func()
}

func NewStackOpTracer(rwCounter *RwCounter, txnId uint) *StackOpTracer {
	return &StackOpTracer{
		rwCtr: rwCounter,
		txnId: txnId,
	}
}

type SaveStackElements struct {
	// top-N elements before opcode
	before uint
	// top-N elements after opcode
	after uint
}

var opcodeStackToSave = map[vm.OpCode]SaveStackElements{
	vm.PREVRANDAO:     {0, 1},
	vm.SHL:            {2, 1},
	vm.SHR:            {2, 1},
	vm.SAR:            {2, 1},
	vm.EXTCODEHASH:    {1, 1},
	vm.CREATE2:        {4, 1},
	vm.STATICCALL:     {6, 1},
	vm.RETURNDATASIZE: {0, 1},
	vm.RETURNDATACOPY: {3, 0},
	vm.REVERT:         {2, 0},
	vm.DELEGATECALL:   {6, 1},
	vm.STOP:           {0, 0},
	vm.ADD:            {2, 1},
	vm.MUL:            {2, 1},
	vm.SUB:            {2, 1},
	vm.DIV:            {2, 1},
	vm.SDIV:           {2, 1},
	vm.MOD:            {2, 1},
	vm.SMOD:           {2, 1},
	vm.ADDMOD:         {3, 1},
	vm.MULMOD:         {3, 1},
	vm.EXP:            {2, 1},
	vm.SIGNEXTEND:     {2, 1},
	vm.LT:             {2, 1},
	vm.GT:             {2, 1},
	vm.SLT:            {2, 1},
	vm.SGT:            {2, 1},
	vm.EQ:             {2, 1},
	vm.ISZERO:         {1, 1},
	vm.AND:            {2, 1},
	vm.XOR:            {2, 1},
	vm.OR:             {2, 1},
	vm.NOT:            {1, 1},
	vm.BYTE:           {2, 1},
	vm.KECCAK256:      {2, 1},
	vm.ADDRESS:        {0, 1},
	vm.BALANCE:        {1, 1},
	vm.ORIGIN:         {0, 1},
	vm.CALLER:         {0, 1},
	vm.CALLVALUE:      {0, 1},
	vm.CALLDATALOAD:   {1, 1},
	vm.CALLDATASIZE:   {0, 1},
	vm.CALLDATACOPY:   {3, 0},
	vm.CODESIZE:       {0, 1},
	vm.CODECOPY:       {3, 0},
	vm.GASPRICE:       {0, 1},
	vm.EXTCODESIZE:    {1, 1},
	vm.EXTCODECOPY:    {4, 0},
	vm.BLOCKHASH:      {1, 1},
	vm.COINBASE:       {0, 1},
	vm.TIMESTAMP:      {0, 1},
	vm.NUMBER:         {0, 1},
	vm.GASLIMIT:       {0, 1},

	// this value differs from jumptable as far as POP is not a read in terms of circuit.
	// while migrating to values from jumptable we need also to introduce some new characteristic of opcode determining
	// if popped stack item is read by anybody or not
	vm.POP: {0, 0},

	vm.MLOAD:        {1, 1},
	vm.MSTORE:       {2, 0},
	vm.MSTORE8:      {2, 0},
	vm.MCOPY:        {3, 0},
	vm.SLOAD:        {1, 1},
	vm.SSTORE:       {2, 0},
	vm.JUMP:         {1, 0},
	vm.JUMPI:        {2, 0},
	vm.PC:           {0, 1},
	vm.MSIZE:        {0, 1},
	vm.GAS:          {0, 1},
	vm.JUMPDEST:     {0, 0},
	vm.PUSH0:        {0, 1},
	vm.PUSH1:        {0, 1},
	vm.PUSH2:        {0, 1},
	vm.PUSH3:        {0, 1},
	vm.PUSH4:        {0, 1},
	vm.PUSH5:        {0, 1},
	vm.PUSH6:        {0, 1},
	vm.PUSH7:        {0, 1},
	vm.PUSH8:        {0, 1},
	vm.PUSH9:        {0, 1},
	vm.PUSH10:       {0, 1},
	vm.PUSH11:       {0, 1},
	vm.PUSH12:       {0, 1},
	vm.PUSH13:       {0, 1},
	vm.PUSH14:       {0, 1},
	vm.PUSH15:       {0, 1},
	vm.PUSH16:       {0, 1},
	vm.PUSH17:       {0, 1},
	vm.PUSH18:       {0, 1},
	vm.PUSH19:       {0, 1},
	vm.PUSH20:       {0, 1},
	vm.PUSH21:       {0, 1},
	vm.PUSH22:       {0, 1},
	vm.PUSH23:       {0, 1},
	vm.PUSH24:       {0, 1},
	vm.PUSH25:       {0, 1},
	vm.PUSH26:       {0, 1},
	vm.PUSH27:       {0, 1},
	vm.PUSH28:       {0, 1},
	vm.PUSH29:       {0, 1},
	vm.PUSH30:       {0, 1},
	vm.PUSH31:       {0, 1},
	vm.PUSH32:       {0, 1},
	vm.LOG0:         {2, 0},
	vm.LOG1:         {3, 0},
	vm.LOG2:         {4, 0},
	vm.LOG3:         {5, 0},
	vm.LOG4:         {6, 0},
	vm.CREATE:       {3, 1},
	vm.CALL:         {7, 1},
	vm.CALLCODE:     {7, 1},
	vm.RETURN:       {2, 0},
	vm.SELFDESTRUCT: {1, 0},
}

func (sot *StackOpTracer) traceBasicOp() bool {
	stackToSave, ok := opcodeStackToSave[sot.opCode]
	if !ok {
		return false
	}

	// save top n elements before operation
	stack := NewStackAccessor(sot.scope.StackData())
	for i := range stackToSave.before {
		el, idx := stack.BackWIndex(int(i))
		sot.res = append(sot.res, StackOp{
			IsRead: true,
			Idx:    idx,
			Value:  types.Uint256(*el),
			PC:     sot.pc,
			RwIdx:  sot.rwCtr.NextIdx(),
			TxnId:  sot.txnId,
		})
	}

	// save top n elements after operation
	sot.prevOpFinisher = func() {
		stack := NewStackAccessor(sot.scope.StackData())
		for i := range stackToSave.after {
			el, idx := stack.BackWIndex(int(i))
			sot.res = append(sot.res, StackOp{
				IsRead: false,
				Idx:    idx,
				Value:  types.Uint256(*el),
				PC:     sot.pc,
				RwIdx:  sot.rwCtr.NextIdx(),
				TxnId:  sot.txnId,
			})
		}
	}
	return true
}

// We don't need to have a copy of entire stack for DUP and SWAP opcodes. Save only two elements for each.
type ElementToSeek uint

// Read (size - dupN) element, push on stack
var dupToSeekStack = map[vm.OpCode]ElementToSeek{
	vm.DUP1:  0,
	vm.DUP2:  1,
	vm.DUP3:  2,
	vm.DUP4:  3,
	vm.DUP5:  4,
	vm.DUP6:  5,
	vm.DUP7:  6,
	vm.DUP8:  7,
	vm.DUP9:  8,
	vm.DUP10: 9,
	vm.DUP11: 10,
	vm.DUP12: 11,
	vm.DUP13: 12,
	vm.DUP14: 13,
	vm.DUP15: 14,
	vm.DUP16: 15,
}

func (sot *StackOpTracer) traceDupOp() bool {
	seekStack, ok := dupToSeekStack[sot.opCode]
	if !ok {
		return false
	}
	stack := NewStackAccessor(sot.scope.StackData())
	el, idx := stack.BackWIndex(int(seekStack))
	sot.res = append(sot.res, StackOp{
		IsRead: true,
		Idx:    idx,
		Value:  types.Uint256(*el),
		PC:     sot.pc,
		RwIdx:  sot.rwCtr.NextIdx(),
		TxnId:  sot.txnId,
	})

	sot.prevOpFinisher = func() {
		stack := NewStackAccessor(sot.scope.StackData())
		el, idx := stack.PopWIndex()
		sot.res = append(sot.res, StackOp{
			IsRead: false,
			Idx:    idx,
			Value:  types.Uint256(*el),
			PC:     sot.pc,
			RwIdx:  sot.rwCtr.NextIdx(),
			TxnId:  sot.txnId,
		})
	}
	return true
}

// Read (size - swapN), (size - 1) elements, write to the same positions
var swapToSeekStack = map[vm.OpCode]ElementToSeek{
	vm.SWAP1:  1,
	vm.SWAP2:  2,
	vm.SWAP3:  3,
	vm.SWAP4:  4,
	vm.SWAP5:  5,
	vm.SWAP6:  6,
	vm.SWAP7:  7,
	vm.SWAP8:  8,
	vm.SWAP9:  9,
	vm.SWAP10: 10,
	vm.SWAP11: 11,
	vm.SWAP12: 12,
	vm.SWAP13: 13,
	vm.SWAP14: 14,
	vm.SWAP15: 15,
	vm.SWAP16: 16,
}

type swapElem struct {
	val types.Uint256
	pos int
}

func (sot *StackOpTracer) getSwappingElements(offset ElementToSeek) (top, swap swapElem) {
	stack := NewStackAccessor(sot.scope.StackData())

	var ptr *uint256.Int

	ptr, top.pos = stack.BackWIndex(0)
	top.val = types.Uint256(*ptr)

	ptr, swap.pos = stack.BackWIndex(int(offset))
	swap.val = types.Uint256(*ptr)
	return
}

func (sot *StackOpTracer) traceSwapOp() bool {
	seekStack, ok := swapToSeekStack[sot.opCode]
	if !ok {
		return false
	}

	top, swap := sot.getSwappingElements(seekStack)
	sot.res = append(sot.res,
		StackOp{
			IsRead: true,
			Idx:    top.pos,
			Value:  top.val,
			PC:     sot.pc,
			RwIdx:  sot.rwCtr.NextIdx(),
			TxnId:  sot.txnId,
		},
		StackOp{
			IsRead: true,
			Idx:    swap.pos,
			Value:  swap.val,
			PC:     sot.pc,
			RwIdx:  sot.rwCtr.NextIdx(),
			TxnId:  sot.txnId,
		},
	)

	sot.prevOpFinisher = func() {
		top, swap := sot.getSwappingElements(seekStack)
		sot.res = append(sot.res,
			StackOp{
				IsRead: false,
				Idx:    swap.pos,
				Value:  swap.val,
				PC:     sot.pc,
				RwIdx:  sot.rwCtr.NextIdx(),
				TxnId:  sot.txnId,
			},
			StackOp{
				IsRead: false,
				Idx:    top.pos,
				Value:  top.val,
				PC:     sot.pc,
				RwIdx:  sot.rwCtr.NextIdx(),
				TxnId:  sot.txnId,
			},
		)
	}

	return true
}

func (sot *StackOpTracer) TraceOp(opCode vm.OpCode, pc uint64, scope tracing.OpContext) error {
	if sot.prevOpFinisher != nil {
		return ErrTraceNotFinalized
	}
	sot.opCode = opCode
	sot.pc = pc
	sot.scope = scope

	if !sot.traceBasicOp() && !sot.traceDupOp() && !sot.traceSwapOp() {
		return fmt.Errorf("no stack save info for opcode: %v", opCode)
	}
	return nil
}

func (sot *StackOpTracer) FinishPrevOpcodeTracing() {
	if sot.prevOpFinisher == nil {
		// first opcode for the tracer
		return
	}

	sot.prevOpFinisher()
	sot.prevOpFinisher = nil
}

func (sot *StackOpTracer) Finalize() []StackOp {
	// The last opcode could be one of STOP, RETURN, REVERT, SELFDESTRUCT. Each of
	// them doesn't put anything on stack.
	sot.FinishPrevOpcodeTracing()
	return sot.res
}
