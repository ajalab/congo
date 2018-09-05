package congo

import (
	// #cgo LDFLAGS: -lz3
	// #include <stdlib.h>
	// #include <z3.h>
	"C"
)
import (
	"errors"
	"fmt"
	"go/token"
	"go/types"
	"log"

	"golang.org/x/tools/go/ssa"
)

type Z3ConstraintSet struct {
	asts map[ssa.Value]C.Z3_ast
	ctx  C.Z3_context

	currentBlock *ssa.BasicBlock
	prevBlock    *ssa.BasicBlock

	assertions []*assertion
	symbols    []*symbol
}

type assertion struct {
	instr ssa.Instruction
	cond  C.Z3_ast
	orig  bool
}

type symbol struct {
	z3  C.Z3_ast
	ssa ssa.Value
}

func NewZ3ConstraintSet() *Z3ConstraintSet {
	cfg := C.Z3_mk_config()
	defer C.Z3_del_config(cfg)

	ctx := C.Z3_mk_context(cfg)

	return &Z3ConstraintSet{
		asts: make(map[ssa.Value]C.Z3_ast),
		ctx:  ctx,
	}
}

func (cs *Z3ConstraintSet) Close() {
	C.Z3_del_context(cs.ctx)
}

func (cs *Z3ConstraintSet) addSymbol(ssaSymbol ssa.Value) error {
	var v C.Z3_ast
	symbolID := C.Z3_mk_int_symbol(cs.ctx, C.int(len(cs.symbols)))

	switch ty := ssaSymbol.Type().(type) {
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
			v = C.Z3_mk_const(cs.ctx, symbolID, sort)
		default:
			return fmt.Errorf("unsupported symbol basic kind: %v", ty.Kind())
		}
	default:
		return fmt.Errorf("unsupported symbol type: %T", ty)
	}
	if v != nil {
		cs.asts[ssaSymbol] = v
	}

	cs.symbols = append(cs.symbols, &symbol{
		ssa: ssaSymbol,
		z3:  v,
	})

	return nil
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
		if x == nil || y == nil {
			return
		}
		args := []C.Z3_ast{x, y}
		switch instr.Op {
		case token.ADD:
			v = C.Z3_mk_add(cs.ctx, 2, &args[0])
		case token.SUB:
			v = C.Z3_mk_sub(cs.ctx, 2, &args[0])
		case token.MUL:
			v = C.Z3_mk_mul(cs.ctx, 2, &args[0])
		case token.QUO:
			v = C.Z3_mk_div(cs.ctx, x, y)
		case token.EQL:
			v = C.Z3_mk_eq(cs.ctx, x, y)
		case token.LSS:
			v = C.Z3_mk_lt(cs.ctx, x, y)
		case token.LEQ:
			v = C.Z3_mk_le(cs.ctx, x, y)
		case token.GTR:
			v = C.Z3_mk_gt(cs.ctx, x, y)
		case token.GEQ:
			v = C.Z3_mk_ge(cs.ctx, x, y)
		case token.LAND:
			v = C.Z3_mk_and(cs.ctx, 2, &args[0])
		case token.LOR:
			v = C.Z3_mk_or(cs.ctx, 2, &args[0])

		default:
			log.Fatalln("addConstraint: Not implemented BinOp: ", instr)
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
	case *ssa.Call:
		fn, ok := instr.Call.Value.(*ssa.Function)
		if !ok {
			return
		}
		for i, arg := range instr.Call.Args {
			ast := cs.get(arg)
			cs.asts[fn.Params[i]] = ast
		}
	}

}

func (cs *Z3ConstraintSet) addAssertion(ifInstr *ssa.If, orig bool) {
	v := ifInstr.Cond
	if cond, ok := cs.asts[v]; ok {
		assert := &assertion{
			instr: ifInstr,
			cond:  cond,
			orig:  orig,
		}
		cs.assertions = append(cs.assertions, assert)
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
	return nil
	// log.Fatalln("get: Corresponding Z3 AST was not found", v)
	// panic("unimplemented")
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
	log.Fatalln("getZ3ConstAST: Unimplemented const value", v)
	panic("unimplemented")
}

func (cs *Z3ConstraintSet) solve(negateAssertion int) ([]interface{}, error) {
	solver := C.Z3_mk_solver(cs.ctx)
	C.Z3_solver_inc_ref(cs.ctx, solver)
	defer C.Z3_solver_dec_ref(cs.ctx, solver)
	for i := 0; i < negateAssertion; i++ {
		assert := cs.assertions[i]
		cond := assert.cond
		if !assert.orig {
			cond = C.Z3_mk_not(cs.ctx, cond)
		}
		C.Z3_solver_assert(cs.ctx, solver, cond)
	}

	negAssert := cs.assertions[negateAssertion]
	negCond := negAssert.cond
	if negAssert.orig {
		negCond = C.Z3_mk_not(cs.ctx, negCond)
	}
	C.Z3_solver_assert(cs.ctx, solver, negCond)

	result := C.Z3_solver_check(cs.ctx, solver)

	var err error
	switch result {
	case C.Z3_L_FALSE:
		err = errors.New("unsat")
	case C.Z3_L_TRUE:
		m := C.Z3_solver_get_model(cs.ctx, solver)
		if m != nil {
			C.Z3_model_inc_ref(cs.ctx, m)
			defer C.Z3_model_dec_ref(cs.ctx, m)
		}
		values, err := cs.getSymbolValues(m)
		if err != nil {
			return nil, err
		}
		return values, nil
	}
	return nil, err
}

func (cs *Z3ConstraintSet) getSymbolValues(m C.Z3_model) ([]interface{}, error) {
	values := make([]interface{}, len(cs.symbols))
	for i, symbol := range cs.symbols {
		values[i] = zero(symbol.ssa.Type())
	}

	n := int(C.Z3_model_get_num_consts(cs.ctx, m))
	for i := 0; i < n; i++ {
		constDecl := C.Z3_model_get_const_decl(cs.ctx, m, C.uint(i))
		symbolID := C.Z3_get_decl_name(cs.ctx, constDecl)
		if k := C.Z3_get_symbol_kind(cs.ctx, symbolID); k != C.Z3_INT_SYMBOL {
			return nil, errors.New("Z3_symbol should be int value")
		}
		idx := int(C.Z3_get_symbol_int(cs.ctx, symbolID))

		a := C.Z3_mk_app(cs.ctx, constDecl, 0, nil)
		var ast C.Z3_ast
		ok := C.Z3_model_eval(cs.ctx, m, a, C.bool(true), &ast)
		if !C.bool(ok) {
			return nil, fmt.Errorf("failed to get symbol[%d] from the model", i)
		}

		v, err := cs.astToValue(ast, cs.symbols[idx].ssa.Type())
		if err != nil {
			return nil, err
		}

		values[idx] = v
	}

	return values, nil
}

func (cs *Z3ConstraintSet) astToValue(ast C.Z3_ast, ty types.Type) (interface{}, error) {
	switch C.Z3_get_ast_kind(cs.ctx, ast) {
	case C.Z3_NUMERAL_AST:
		basicTy, ok := ty.(*types.Basic)
		if !ok {
			return nil, fmt.Errorf("illegal type")
		}
		var u C.uint64_t
		ok = bool(C.Z3_get_numeral_uint64(cs.ctx, ast, &u))
		if !ok {
			return nil, fmt.Errorf("Z3_get_numeral_uint64: could not get a uint64 representation of the AST")
		}
		switch basicTy.Kind() {
		case types.Int:
			return int(u), nil
		case types.Int8:
			return int8(u), nil
		case types.Int16:
			return int16(u), nil
		case types.Int32:
			return int32(u), nil
		case types.Int64:
			return int64(u), nil
		case types.Uint:
			return uint(u), nil
		case types.Uint8:
			return uint8(u), nil
		case types.Uint16:
			return uint16(u), nil
		case types.Uint32:
			return uint32(u), nil
		case types.Uint64:
			return uint64(u), nil
		}

	}
	return nil, fmt.Errorf("cannot convert Z3_AST of type %s", ty)
}

func fromTrace(symbols []ssa.Value, traces [][]*ssa.BasicBlock) *Z3ConstraintSet {
	cs := NewZ3ConstraintSet()

	for _, symbol := range symbols {
		cs.addSymbol(symbol)
	}
	for _, trace := range traces {
		for i, block := range trace {
			for _, instr := range block.Instrs {
				cs.addConstraint(instr)
			}
			lastInstr := block.Instrs[len(block.Instrs)-1]
			if ifInstr, ok := lastInstr.(*ssa.If); ok {
				orig := block.Succs[0] == trace[i+1]
				cs.addAssertion(ifInstr, orig)
			}
		}
	}

	return cs
}
