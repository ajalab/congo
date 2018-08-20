package congo

import (
	// #cgo LDFLAGS: -lz3
	// #include <stdlib.h>
	// #include <z3.h>
	"C"
)
import (
	"fmt"
	"go/token"
	"go/types"
	"unsafe"

	"golang.org/x/tools/go/ssa"
)

type Z3ConstraintSet struct {
	asts   map[ssa.Value]C.Z3_ast
	ctx    C.Z3_context
	solver C.Z3_solver

	currentBlock *ssa.BasicBlock
	prevBlock    *ssa.BasicBlock
}

func NewZ3ConstraintSet() *Z3ConstraintSet {
	cfg := C.Z3_mk_config()
	defer C.Z3_del_config(cfg)

	ctx := C.Z3_mk_context(cfg)
	solver := C.Z3_mk_solver(ctx)
	C.Z3_solver_inc_ref(ctx, solver)

	return &Z3ConstraintSet{
		asts:   make(map[ssa.Value]C.Z3_ast),
		ctx:    ctx,
		solver: solver,
	}
}

func (cs *Z3ConstraintSet) Close() {
	C.Z3_solver_dec_ref(cs.ctx, cs.solver)
	C.Z3_del_context(cs.ctx)
}

func (cs *Z3ConstraintSet) addParameter(param *ssa.Parameter) {
	var v C.Z3_ast = nil
	cname := C.CString("congo_param_" + param.Name())
	defer C.free(unsafe.Pointer(cname))
	symbol := C.Z3_mk_string_symbol(cs.ctx, cname)

	switch ty := param.Type().(type) {
	case *types.Basic:
		switch ty.Kind() {
		case types.Int:
			fallthrough
		case types.Int8:
			fallthrough
		case types.Int16:
			fallthrough
		case types.Int32:
			fallthrough
		case types.Int64:
			sort := C.Z3_mk_int_sort(cs.ctx)
			v = C.Z3_mk_const(cs.ctx, symbol, sort)
		}
	}
	if v != nil {
		cs.asts[param] = v
	}
}

func (cs *Z3ConstraintSet) addConstraint(instr ssa.Instruction) {
	block := instr.Block()
	if cs.currentBlock != block {
		cs.prevBlock = cs.currentBlock
		cs.currentBlock = block
	}

	switch instr := instr.(type) {
	case *ssa.BinOp:
		var v C.Z3_ast
		x := cs.get(instr.X)
		y := cs.get(instr.Y)
		args := []C.Z3_ast{x, y}
		switch instr.Op {
		case token.ADD:
			v = C.Z3_mk_add(cs.ctx, 2, &args[0])
		case token.LSS:
			v = C.Z3_mk_lt(cs.ctx, x, y)
		case token.LEQ:
			v = C.Z3_mk_le(cs.ctx, x, y)
		case token.GTR:
			v = C.Z3_mk_gt(cs.ctx, x, y)
		case token.GEQ:
			v = C.Z3_mk_ge(cs.ctx, x, y)

		default:
			panic("unimplemented")
		}
		cs.asts[instr] = v
	case *ssa.Phi:
		var v C.Z3_ast
		for i, pred := range instr.Block().Preds {
			if pred == cs.prevBlock {
				// TODO(ajalab) New variable?
				v = cs.get(instr.Edges[i])
				break
			}
		}
		cs.asts[instr] = v
	}
}

func (cs *Z3ConstraintSet) addAssertion(v ssa.Value) {
	if a, ok := cs.asts[v]; ok {
		C.Z3_solver_assert(cs.ctx, cs.solver, a)
	}
}

func (cs *Z3ConstraintSet) get(v ssa.Value) C.Z3_ast {
	switch v := v.(type) {
	case *ssa.Const:
		return cs.getZ3ConstAST(v)
	}

	if a, ok := cs.asts[v]; ok {
		return a
	}
	panic("unimplemented")
}

func (cs *Z3ConstraintSet) getZ3ConstAST(v *ssa.Const) C.Z3_ast {
	switch ty := v.Type().(type) {
	case *types.Basic:
		switch ty.Kind() {
		case types.Int:
			fallthrough
		case types.Int8:
			fallthrough
		case types.Int16:
			fallthrough
		case types.Int32:
			fallthrough
		case types.Int64:
			sort := C.Z3_mk_int_sort(cs.ctx)
			return C.Z3_mk_int(cs.ctx, C.int(v.Int64()), sort)
		}
	}
	panic("unimplemented")
}

func (cs *Z3ConstraintSet) solve() {
	result := C.Z3_solver_check(cs.ctx, cs.solver)

	switch result {
	case C.Z3_L_FALSE:
		fmt.Println("unsat")
	case C.Z3_L_TRUE:
		fmt.Println("sat")
		m := C.Z3_solver_get_model(cs.ctx, cs.solver)
		if m != nil {
			C.Z3_model_inc_ref(cs.ctx, m)
		}
		fmt.Printf("%s\n", C.GoString(C.Z3_model_to_string(cs.ctx, m)))
		if m != nil {
			C.Z3_model_dec_ref(cs.ctx, m)
		}
	}
}

func fromTrace(targetFunc *ssa.Function, trace []*ssa.BasicBlock) *Z3ConstraintSet {
	cs := NewZ3ConstraintSet()
	defer cs.Close()

	for _, param := range targetFunc.Params {
		cs.addParameter(param)
	}
	for _, block := range trace {
		fmt.Printf(".%d:\n", block.Index)
		for _, instr := range block.Instrs {
			fmt.Printf("%[1]v: %[1]T\n", instr)
			cs.addConstraint(instr)
			if ifInstr, ok := instr.(*ssa.If); ok {
				cs.addAssertion(ifInstr.Cond)
			}
		}
	}
	cs.solve()
	return nil
}
