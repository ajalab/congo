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
	"log"

	"github.com/pkg/errors"
	"golang.org/x/tools/go/ssa"
)

type Z3Solver struct {
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

type UnsatError struct{}

func (ue UnsatError) Error() string {
	return "unsat"
}

func NewZ3Solver(symbols []ssa.Value, trace []*ssa.BasicBlock) *Z3Solver {
	cfg := C.Z3_mk_config()
	defer C.Z3_del_config(cfg)

	ctx := C.Z3_mk_context(cfg)
	s := &Z3Solver{
		asts: make(map[ssa.Value]C.Z3_ast),
		ctx:  ctx,
	}

	for _, symbol := range symbols {
		s.addSymbol(symbol)
	}
	for i, block := range trace {
		for _, instr := range block.Instrs {
			s.addConstraint(instr)
		}
		lastInstr := block.Instrs[len(block.Instrs)-1]
		if ifInstr, ok := lastInstr.(*ssa.If); ok {
			orig := block.Succs[0] == trace[i+1]
			s.addAssertion(ifInstr, orig)
		}
	}

	return s
}

func (s *Z3Solver) Close() {
	C.Z3_del_context(s.ctx)
}

func (s *Z3Solver) addSymbol(ssaSymbol ssa.Value) error {
	var v C.Z3_ast
	symbolID := C.Z3_mk_int_symbol(s.ctx, C.int(len(s.symbols)))

	switch ty := ssaSymbol.Type().(type) {
	case *types.Basic:
		info := ty.Info()
		switch {
		case info&types.IsInteger > 0:
			sort := C.Z3_mk_bv_sort(s.ctx, C.uint(sizeOfBasicKind(ty.Kind())))
			v = C.Z3_mk_const(s.ctx, symbolID, sort)
		default:
			return fmt.Errorf("unsupported basic type: %v", ty)
		}
	default:
		return fmt.Errorf("unsupported symbol type: %T", ty)
	}
	if v != nil {
		s.asts[ssaSymbol] = v
	}

	s.symbols = append(s.symbols, &symbol{
		ssa: ssaSymbol,
		z3:  v,
	})

	return nil
}

func z3MakeAdd(ctx C.Z3_context, x, y C.Z3_ast, ty types.Type) C.Z3_ast {
	basicTy, ok := ty.(*types.Basic)
	if !ok {
		log.Fatalf("z3MakeAdd: invalid type: %T\n", ty)
		panic("unreachable")
	}
	info := basicTy.Info()
	switch {
	case info&types.IsInteger > 0:
		return C.Z3_mk_bvadd(ctx, x, y)
	default:
		log.Fatalf("z3MakeAdd: not implemented: %T\n", ty)
		panic("unimplemented")
	}
}

func z3MakeSub(ctx C.Z3_context, x, y C.Z3_ast, ty types.Type) C.Z3_ast {
	basicTy, ok := ty.(*types.Basic)
	if !ok {
		log.Fatalf("z3MakeSub: invalid type: %T\n", ty)
		panic("unreachable")
	}
	info := basicTy.Info()
	switch {
	case info&types.IsInteger > 0:
		return C.Z3_mk_bvsub(ctx, x, y)
	default:
		log.Fatalf("z3MakeSub: not implemented: %T\n", ty)
		panic("unimplemented")
	}
}

func z3MakeMul(ctx C.Z3_context, x, y C.Z3_ast, ty types.Type) C.Z3_ast {
	basicTy, ok := ty.(*types.Basic)
	if !ok {
		log.Fatalf("z3MakeMul: invalid type: %T\n", ty)
		panic("unreachable")
	}
	info := basicTy.Info()
	switch {
	case info&types.IsInteger > 0:
		return C.Z3_mk_bvmul(ctx, x, y)
	default:
		log.Fatalf("z3MakeMul: not implemented: %T\n", ty)
		panic("unimplemented")
	}
}

func z3MakeDiv(ctx C.Z3_context, x, y C.Z3_ast, ty types.Type) C.Z3_ast {
	basicTy, ok := ty.(*types.Basic)
	if !ok {
		log.Fatalf("z3MakeDiv: invalid type: %T\n", ty)
		panic("unreachable")
	}
	info := basicTy.Info()
	switch {
	case info&types.IsInteger > 0:
		if info&types.IsUnsigned > 0 {
			return C.Z3_mk_bvudiv(ctx, x, y)
		} else {
			return C.Z3_mk_bvsdiv(ctx, x, y)
		}
	default:
		log.Fatalf("z3MakeDiv: not implemented info: %v", basicTy.Kind())
		panic("unimplemented")
	}
}

func z3MakeLt(ctx C.Z3_context, x, y C.Z3_ast, ty types.Type) C.Z3_ast {
	basicTy, ok := ty.(*types.Basic)
	if !ok {
		log.Fatalf("z3MakeLt: invalid type: %T\n", ty)
		panic("unreachable")
	}
	info := basicTy.Info()
	switch {
	case info&types.IsInteger > 0:
		if info&types.IsUnsigned > 0 {
			return C.Z3_mk_bvult(ctx, x, y)
		} else {
			return C.Z3_mk_bvslt(ctx, x, y)
		}
	default:
		log.Fatalf("z3MakeLt: not implemented info: %v", basicTy.Kind())
		panic("unimplemented")
	}
}

func z3MakeLe(ctx C.Z3_context, x, y C.Z3_ast, ty types.Type) C.Z3_ast {
	basicTy, ok := ty.(*types.Basic)
	if !ok {
		log.Fatalf("z3MakeLe: invalid type: %T\n", ty)
		panic("unreachable")
	}
	info := basicTy.Info()
	switch {
	case info&types.IsInteger > 0:
		if info&types.IsUnsigned > 0 {
			return C.Z3_mk_bvule(ctx, x, y)
		} else {
			return C.Z3_mk_bvsle(ctx, x, y)
		}
	default:
		log.Fatalf("z3MakeLe: not implemented info: %v", basicTy.Kind())
		panic("unimplemented")
	}
}

func z3MakeGt(ctx C.Z3_context, x, y C.Z3_ast, ty types.Type) C.Z3_ast {
	basicTy, ok := ty.(*types.Basic)
	if !ok {
		log.Fatalf("z3MakeGt: invalid type: %T\n", ty)
		panic("unreachable")
	}
	info := basicTy.Info()
	switch {
	case info&types.IsInteger > 0:
		if info&types.IsUnsigned > 0 {
			return C.Z3_mk_bvugt(ctx, x, y)
		} else {
			return C.Z3_mk_bvsgt(ctx, x, y)
		}
	default:
		log.Fatalf("z3MakeGt: not implemented info: %v", basicTy.Kind())
		panic("unimplemented")
	}
}

func z3MakeGe(ctx C.Z3_context, x, y C.Z3_ast, ty types.Type) C.Z3_ast {
	basicTy, ok := ty.(*types.Basic)
	if !ok {
		log.Fatalf("z3MakeGe: invalid type: %T\n", ty)
		panic("unreachable")
	}
	info := basicTy.Info()
	switch {
	case info&types.IsInteger > 0:
		if info&types.IsUnsigned > 0 {
			return C.Z3_mk_bvuge(ctx, x, y)
		} else {
			return C.Z3_mk_bvsge(ctx, x, y)
		}
	default:
		log.Fatalf("z3MakeGe: not implemented info: %v", basicTy.Kind())
		panic("unimplemented")
	}
}

func (s *Z3Solver) addConstraint(instr ssa.Instruction) {
	block := instr.Block()
	if s.currentBlock != block {
		s.prevBlock = s.currentBlock
		s.currentBlock = block
	}

	switch instr := instr.(type) {
	case *ssa.BinOp:
		var v C.Z3_ast
		x := s.get(instr.X)
		y := s.get(instr.Y)
		ty := instr.X.Type()
		if x == nil || y == nil {
			return
		}
		args := []C.Z3_ast{x, y}
		switch instr.Op {
		case token.ADD:
			v = z3MakeAdd(s.ctx, x, y, ty)
		case token.SUB:
			v = z3MakeSub(s.ctx, x, y, ty)
		case token.MUL:
			v = z3MakeMul(s.ctx, x, y, ty)
		case token.QUO:
			v = z3MakeDiv(s.ctx, x, y, ty)
		case token.EQL:
			v = C.Z3_mk_eq(s.ctx, x, y)
		case token.LSS:
			v = z3MakeLt(s.ctx, x, y, ty)
		case token.LEQ:
			v = z3MakeLe(s.ctx, x, y, ty)
		case token.GTR:
			v = z3MakeGt(s.ctx, x, y, ty)
		case token.GEQ:
			v = z3MakeGe(s.ctx, x, y, ty)
		case token.LAND:
			v = C.Z3_mk_and(s.ctx, 2, &args[0])
		case token.LOR:
			v = C.Z3_mk_or(s.ctx, 2, &args[0])

		default:
			log.Fatalln("addConstraint: Not implemented BinOp: ", instr)
			panic("unimplemented")
		}
		s.asts[instr] = v
	case *ssa.Phi:
		var v C.Z3_ast
		for i, pred := range instr.Block().Preds {
			if pred == s.prevBlock {
				// TODO(ajalab) New variable?
				v = s.get(instr.Edges[i])
				break
			}
		}
		s.asts[instr] = v
	case *ssa.Call:
		fn, ok := instr.Call.Value.(*ssa.Function)
		if !ok {
			return
		}
		for i, arg := range instr.Call.Args {
			ast := s.get(arg)
			s.asts[fn.Params[i]] = ast
		}
	}

}

func (s *Z3Solver) addAssertion(ifInstr *ssa.If, orig bool) {
	v := ifInstr.Cond
	if cond, ok := s.asts[v]; ok {
		assert := &assertion{
			instr: ifInstr,
			cond:  cond,
			orig:  orig,
		}
		s.assertions = append(s.assertions, assert)
	}
}

func (s *Z3Solver) get(v ssa.Value) C.Z3_ast {
	switch v := v.(type) {
	case *ssa.Const:
		return s.getZ3ConstAST(v)
	}

	if a, ok := s.asts[v]; ok {
		return a
	}
	return nil
	// log.Fatalln("get: Corresponding Z3 AST was not found", v)
	// panic("unimplemented")
}

func (s *Z3Solver) getZ3ConstAST(v *ssa.Const) C.Z3_ast {
	switch ty := v.Type().(type) {
	case *types.Basic:
		switch ty.Info() {
		case types.IsInteger:
			size := sizeOfBasicKind(ty.Kind())
			sort := C.Z3_mk_bv_sort(s.ctx, C.uint(size))
			return C.Z3_mk_int(s.ctx, C.int(v.Int64()), sort)
		case types.IsUnsigned:
			size := sizeOfBasicKind(ty.Kind())
			sort := C.Z3_mk_bv_sort(s.ctx, C.uint(size))
			return C.Z3_mk_unsigned_int(s.ctx, C.uint(v.Uint64()), sort)
		}
	}
	log.Fatalln("getZ3ConstAST: Unimplemented const value", v)
	panic("unimplemented")
}

func (s *Z3Solver) solve(negateAssertion int) ([]interface{}, error) {
	solver := C.Z3_mk_solver(s.ctx)
	C.Z3_solver_inc_ref(s.ctx, solver)
	defer C.Z3_solver_dec_ref(s.ctx, solver)
	for i := 0; i < negateAssertion; i++ {
		assert := s.assertions[i]
		cond := assert.cond
		if !assert.orig {
			cond = C.Z3_mk_not(s.ctx, cond)
		}
		C.Z3_solver_assert(s.ctx, solver, cond)
	}

	negAssert := s.assertions[negateAssertion]
	negCond := negAssert.cond
	if negAssert.orig {
		negCond = C.Z3_mk_not(s.ctx, negCond)
	}
	C.Z3_solver_assert(s.ctx, solver, negCond)

	result := C.Z3_solver_check(s.ctx, solver)

	switch result {
	case C.Z3_L_FALSE:
		return nil, UnsatError{}
	case C.Z3_L_TRUE:
		m := C.Z3_solver_get_model(s.ctx, solver)
		if m != nil {
			C.Z3_model_inc_ref(s.ctx, m)
			defer C.Z3_model_dec_ref(s.ctx, m)
		}
		values, err := s.getSymbolValues(m)
		if err != nil {
			return nil, errors.Wrapf(err, "solve: failed to get values from a model: %s", C.GoString(C.Z3_model_to_string(s.ctx, m)))
		}
		return values, nil
	default:
		return nil, fmt.Errorf("failed to solve: %s", C.GoString(C.Z3_solver_to_string(s.ctx, solver)))
	}
}

func (s *Z3Solver) getSymbolValues(m C.Z3_model) ([]interface{}, error) {
	values := make([]interface{}, len(s.symbols))
	for i, symbol := range s.symbols {
		values[i] = zero(symbol.ssa.Type())
	}

	n := int(C.Z3_model_get_num_consts(s.ctx, m))
	for i := 0; i < n; i++ {
		constDecl := C.Z3_model_get_const_decl(s.ctx, m, C.uint(i))
		symbolID := C.Z3_get_decl_name(s.ctx, constDecl)
		if k := C.Z3_get_symbol_kind(s.ctx, symbolID); k != C.Z3_INT_SYMBOL {
			return nil, errors.New("Z3_symbol should be int value")
		}
		idx := int(C.Z3_get_symbol_int(s.ctx, symbolID))

		a := C.Z3_mk_app(s.ctx, constDecl, 0, nil)
		var ast C.Z3_ast
		ok := C.Z3_model_eval(s.ctx, m, a, C.bool(true), &ast)
		if !C.bool(ok) {
			return nil, fmt.Errorf("failed to get symbol[%d] from the model", i)
		}

		v, err := s.astToValue(ast, s.symbols[idx].ssa.Type())
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert Z3 AST to values")
		}

		values[idx] = v
	}

	return values, nil
}

func (s *Z3Solver) astToValue(ast C.Z3_ast, ty types.Type) (interface{}, error) {
	switch C.Z3_get_ast_kind(s.ctx, ast) {
	case C.Z3_NUMERAL_AST:
		basicTy, ok := ty.(*types.Basic)
		if !ok {
			return nil, fmt.Errorf("illegal type")
		}
		var u C.uint64_t
		ok = bool(C.Z3_get_numeral_uint64(s.ctx, ast, &u))
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
