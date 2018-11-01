package solver

import (
	/*
		#cgo LDFLAGS: -lz3
		#include <stdlib.h>
		#include <z3.h>
		extern void goZ3ErrorHandler(Z3_context ctx, Z3_error_code e);
	*/
	"C"
)
import (
	"fmt"
	"go/constant"
	"go/token"
	"go/types"
	"log"
	"strconv"
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/tools/go/ssa"
)

const (
	z3SymbolPrefixForSymbol string = "symbol-"
)

// Z3Solver is a type that holds the Z3 context, assertions, and symbols.
type Z3Solver struct {
	asts map[ssa.Value]C.Z3_ast
	ctx  C.Z3_context

	branches  []Branch
	symbols   []ssa.Value
	datatypes map[string]z3Datatype
}

//export goZ3ErrorHandler
func goZ3ErrorHandler(ctx C.Z3_context, e C.Z3_error_code) {
	msg := C.Z3_get_error_msg(ctx, e)
	panic("Z3 error occurred: " + C.GoString(msg))
}

// NewZ3Solver returns a new Z3Solver.
func NewZ3Solver() *Z3Solver {
	cfg := C.Z3_mk_config()
	defer C.Z3_del_config(cfg)

	// TODO(ajalab): We may have to use Z3_mk_context_rc and manually handle the reference count.
	ctx := C.Z3_mk_context(cfg)
	C.Z3_set_error_handler(ctx, (*C.Z3_error_handler)(C.goZ3ErrorHandler))
	return &Z3Solver{
		asts:      make(map[ssa.Value]C.Z3_ast),
		ctx:       ctx,
		datatypes: make(map[string]z3Datatype),
	}
}

// Close deletes the Z3 context.
func (s *Z3Solver) Close() {
	C.Z3_del_context(s.ctx)
}

func (s *Z3Solver) getSymbolAST(z3SymbolName string, ty types.Type) C.Z3_ast {
	z3SymbolNameC := C.CString(z3SymbolName)
	z3Symbol := C.Z3_mk_string_symbol(s.ctx, z3SymbolNameC)
	C.free(unsafe.Pointer(z3SymbolNameC))

	var sort C.Z3_sort
	switch ty := ty.(type) {
	case *types.Basic:
		info := ty.Info()
		switch {
		case info&types.IsBoolean > 0:
			sort = C.Z3_mk_bool_sort(s.ctx)
		case info&types.IsInteger > 0:
			sort = C.Z3_mk_bv_sort(s.ctx, C.uint(sizeOfBasicKind(ty.Kind())))
		case info&types.IsString > 0:
			sort = C.Z3_mk_string_sort(s.ctx)
		}
	case *types.Pointer:
		valAST := s.getSymbolAST("p-"+z3SymbolName, ty.Elem())
		valSort := C.Z3_get_sort(s.ctx, valAST)
		datatypeName := fmt.Sprintf("dt-%d", len(s.datatypes))
		datatype := newPointerSort(s.ctx, valSort, datatypeName)
		s.datatypes[ty.String()] = datatype
		sort = datatype.sort
	}

	if sort == nil {
		log.Fatalf("unsupported type: %v", ty)
		panic("unimplemented")
	}
	return C.Z3_mk_const(s.ctx, z3Symbol, sort)
}

// LoadSymbols loads symbolic variables to the solver.
func (s *Z3Solver) LoadSymbols(symbols []ssa.Value) error {
	s.symbols = make([]ssa.Value, len(symbols))
	for i, value := range symbols {
		z3SymbolName := fmt.Sprintf("%s%d", z3SymbolPrefixForSymbol, i)
		ast := s.getSymbolAST(z3SymbolName, value.Type())
		if ast != nil {
			s.asts[value] = ast
		}
	}
	copy(s.symbols, symbols)
	return nil
}

// LoadTrace loads a running trace to the solver.
func (s *Z3Solver) LoadTrace(trace []ssa.Instruction, complete bool) {
	var currentBlock *ssa.BasicBlock
	var prevBlock *ssa.BasicBlock
	var callStack []*ssa.Call
	for i, instr := range trace {
		block := instr.Block()
		if currentBlock != block {
			prevBlock = currentBlock
			currentBlock = block
		}

		switch instr := instr.(type) {
		case *ssa.UnOp:
			var err error
			s.asts[instr], err = s.unop(instr)
			if err != nil {
				log.Println(err)
			}
		case *ssa.BinOp:
			// TODO(ajalab) handle errors
			var err error
			s.asts[instr], err = s.binop(instr)
			if err != nil {
				log.Println(err)
			}
		case *ssa.Phi:
			var v C.Z3_ast
			for j, pred := range instr.Block().Preds {
				if pred == prevBlock {
					// TODO(ajalab) New variable?
					v = s.get(instr.Edges[j])
					break
				}
			}
			s.asts[instr] = v
		case *ssa.Call:
			switch fn := instr.Call.Value.(type) {
			case *ssa.Function:
				// Is the called function recorded?
				if i < len(trace)-1 && trace[i+1].Parent() == fn {
					for j, arg := range instr.Call.Args {
						s.asts[fn.Params[j]] = s.get(arg)
					}
					callStack = append(callStack, instr)
				} else {
				}
			case *ssa.Builtin:
				switch fn.Name() {
				case "len":
					arg := instr.Call.Args[0]
					ast := s.get(arg)
					s.asts[instr] = z3MakeLen(s.ctx, ast, arg.Type())
				}
			default:
				log.Fatalln("addConstraint: Not supported function:", fn)
				panic("unimplemented")
			}
		case *ssa.Return:
			callInstr := callStack[len(callStack)-1]
			// TODO(ajalab) Support multiple return values
			switch len(instr.Results) {
			case 0:
			case 1:
				s.asts[callInstr] = s.get(instr.Results[0])
			default:
				log.Fatalln("multiple return values are not supported")
			}
			callStack = callStack[:len(callStack)-1]
		}
		if ifInstr, ok := instr.(*ssa.If); ok {
			if s.get(ifInstr.Cond) != nil {
				thenBlock := instr.Block().Succs[0]
				nextBlock := trace[i+1].Block()
				s.branches = append(s.branches, &BranchIf{
					instr:     ifInstr,
					Direction: thenBlock == nextBlock,
				})
			}
		}
	}
	// Execution was stopped due to panic
	if !complete {
		causeInstr := trace[len(trace)-1]
		switch instr := causeInstr.(type) {
		case *ssa.UnOp:
			s.branches = append(s.branches, &PanicNilPointerDeref{
				instr: instr,
			})
		}
	}
}

// NumBranches returns the number of branch instructions.
func (s *Z3Solver) NumBranches() int {
	return len(s.branches)
}

// Branch returns the i-th branch instruction.
func (s *Z3Solver) Branch(i int) Branch {
	return s.branches[i]
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
	case info&types.IsString > 0:
		args := []C.Z3_ast{x, y}
		return C.Z3_mk_seq_concat(ctx, 2, &args[0])
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
		}
		return C.Z3_mk_bvsdiv(ctx, x, y)
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
		}
		return C.Z3_mk_bvslt(ctx, x, y)

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
		}
		return C.Z3_mk_bvsle(ctx, x, y)

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
		}
		return C.Z3_mk_bvsgt(ctx, x, y)

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
		}
		return C.Z3_mk_bvsge(ctx, x, y)

	default:
		log.Fatalf("z3MakeGe: not implemented info: %v", basicTy.Kind())
		panic("unimplemented")
	}
}

func z3MakeLen(ctx C.Z3_context, x C.Z3_ast, ty types.Type) C.Z3_ast {
	switch ty := ty.(type) {
	case *types.Basic:
		if ty.Kind() == types.String {
			return C.Z3_mk_int2bv(ctx, strconv.IntSize, C.Z3_mk_seq_length(ctx, x))
		}
	}
	log.Fatalf("z3MakeLen: invalid type: %T\n", ty)
	panic("unimplemented")
}

func (s *Z3Solver) unop(instr *ssa.UnOp) (C.Z3_ast, error) {
	x := s.get(instr.X)
	if x == nil {
		return nil, errors.Errorf("unop: operand is not registered: %v", instr)
	}
	switch instr.Op {
	case token.SUB:
		return C.Z3_mk_bvneg(s.ctx, x), nil
	case token.NOT:
		return C.Z3_mk_not(s.ctx, x), nil
	case token.XOR:
		//case token.MUL:
		// case token.ARROW:
	}
	return nil, errors.Errorf("unop: not implemented: %v", instr)
}

func (s *Z3Solver) binop(instr *ssa.BinOp) (C.Z3_ast, error) {
	x := s.get(instr.X)
	y := s.get(instr.Y)
	ty := instr.X.Type()
	if x == nil {
		return nil, errors.Errorf("binop: left operand is not registered: %v", instr)
	}
	if y == nil {
		return nil, errors.Errorf("binop: right operand is not registered: %v", instr)
	}
	args := []C.Z3_ast{x, y}
	switch instr.Op {
	case token.ADD:
		return z3MakeAdd(s.ctx, x, y, ty), nil
	case token.SUB:
		return z3MakeSub(s.ctx, x, y, ty), nil
	case token.MUL:
		return z3MakeMul(s.ctx, x, y, ty), nil
	case token.QUO:
		return z3MakeDiv(s.ctx, x, y, ty), nil
	case token.EQL:
		return C.Z3_mk_eq(s.ctx, x, y), nil
	case token.LSS:
		return z3MakeLt(s.ctx, x, y, ty), nil
	case token.LEQ:
		return z3MakeLe(s.ctx, x, y, ty), nil
	case token.GTR:
		return z3MakeGt(s.ctx, x, y, ty), nil
	case token.GEQ:
		return z3MakeGe(s.ctx, x, y, ty), nil
	case token.LAND:
		return C.Z3_mk_and(s.ctx, 2, &args[0]), nil
	case token.LOR:
		return C.Z3_mk_or(s.ctx, 2, &args[0]), nil
	default:
		return nil, errors.Errorf("binop: not implemented: %v", instr)
	}
}

func (s *Z3Solver) get(v ssa.Value) C.Z3_ast {
	switch v := v.(type) {
	case *ssa.Const:
		return s.getConstAST(v)
	}

	if a, ok := s.asts[v]; ok {
		return a
	}
	log.Printf("get: Corresponding Z3 AST was not found for %s = %s in %s", v.Name(), v, v.Parent())
	return nil
}

func (s *Z3Solver) getConstAST(v *ssa.Const) C.Z3_ast {
	switch ty := v.Type().(type) {
	case *types.Basic:
		info := ty.Info()
		switch {
		case info&types.IsBoolean > 0:
			if constant.BoolVal(v.Value) {
				return C.Z3_mk_true(s.ctx)
			}
			return C.Z3_mk_false(s.ctx)
		case info&types.IsInteger > 0:
			size := sizeOfBasicKind(ty.Kind())
			if info&types.IsUnsigned > 0 {
				sort := C.Z3_mk_bv_sort(s.ctx, C.uint(size))
				return C.Z3_mk_unsigned_int(s.ctx, C.uint(v.Uint64()), sort)
			}
			sort := C.Z3_mk_bv_sort(s.ctx, C.uint(size))
			return C.Z3_mk_int(s.ctx, C.int(v.Int64()), sort)
		case info&types.IsString > 0:
			return C.Z3_mk_string(s.ctx, C.CString(constant.StringVal(v.Value)))
		}
	}
	log.Fatalln("getZ3ConstAST: Unimplemented const value", v)
	panic("unimplemented")
}

func (s *Z3Solver) getBranchAST(branch Branch, negate bool) C.Z3_ast {
	switch b := branch.(type) {
	case *BranchIf:
		cond := s.get(b.instr.Cond)
		if cond == nil {
			log.Printf("corresponding AST for branching condition was not found: %+v", branch.Instr())
			return nil
		}
		if (!negate && !b.Direction) || (negate && b.Direction) {
			cond = C.Z3_mk_not(s.ctx, cond)
		}
		return cond
	case *PanicNilPointerDeref:
		pointer := b.instr.X
		pointerAST := s.get(pointer)
		if pointerAST == nil {
			log.Printf("corresponding AST for branching condition was not found: %+v", branch.Instr())
			return nil
		}
		pointerDT := s.datatypes[pointer.Type().String()].(*z3PointerDatatype)
		cond := C.Z3_mk_app(s.ctx, pointerDT.isNil, 1, &pointerAST)
		if negate {
			cond = C.Z3_mk_not(s.ctx, cond)
		}
		return cond
	default:
		panic("unimplemented")
	}
}

// Solve solves the assertions and returns concrete values for symbols.
// The condition to solve is p_0 /\ p_1 /\ ... /\ p_(k-1) /\ not(a_k)
// where p_i is a predicate of the i-th branching instruction and k = negate.
func (s *Z3Solver) Solve(negate int) ([]interface{}, error) {
	solver := C.Z3_mk_solver(s.ctx)
	C.Z3_solver_inc_ref(s.ctx, solver)
	defer C.Z3_solver_dec_ref(s.ctx, solver)
	for i := 0; i < negate; i++ {
		branch := s.branches[i]
		cond := s.getBranchAST(branch, false)
		C.Z3_solver_assert(s.ctx, solver, cond)
	}

	negBranch := s.branches[negate]
	negCond := s.getBranchAST(negBranch, true)
	if negCond == nil {
		return nil, errors.Errorf("corresponding AST for branching condition was not found: %+v", negBranch.Instr())
	}
	C.Z3_solver_assert(s.ctx, solver, negCond)
	fmt.Println(C.GoString(C.Z3_solver_to_string(s.ctx, solver)))

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
		fmt.Println(C.GoString(C.Z3_model_to_string(s.ctx, m)))
		values, err := s.getSymbolValues(m)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get values from a model: %s", C.GoString(C.Z3_model_to_string(s.ctx, m)))
		}
		return values, nil
	default:
		return nil, errors.Errorf("failed to solve: %s", C.GoString(C.Z3_solver_to_string(s.ctx, solver)))
	}
}

func (s *Z3Solver) getSymbolValues(m C.Z3_model) ([]interface{}, error) {
	values := make([]interface{}, len(s.symbols))

	n := int(C.Z3_model_get_num_consts(s.ctx, m))
	for i := 0; i < n; i++ {
		constDecl := C.Z3_model_get_const_decl(s.ctx, m, C.uint(i))
		z3Symbol := C.Z3_get_decl_name(s.ctx, constDecl)
		z3SymbolName := C.GoString(C.Z3_get_symbol_string(s.ctx, z3Symbol))
		var idx int
		if _, err := fmt.Sscanf(z3SymbolName, z3SymbolPrefixForSymbol+"%d", &idx); err != nil {
			return nil, errors.Errorf("z3Symbol has an illegal format: %s", z3SymbolName)
		}

		a := C.Z3_mk_app(s.ctx, constDecl, 0, nil)
		var ast C.Z3_ast
		ok := C.Z3_model_eval(s.ctx, m, a, C.bool(true), &ast)
		if !C.bool(ok) {
			return nil, errors.Errorf("failed to get symbol[%d] from the model", i)
		}

		v, err := s.astToValue(ast, s.symbols[idx].Type())
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert Z3 AST to values")
		}

		values[idx] = v
	}

	return values, nil
}

func (s *Z3Solver) astToValue(ast C.Z3_ast, ty types.Type) (interface{}, error) {
	kind := C.Z3_get_ast_kind(s.ctx, ast)
	switch kind {
	case C.Z3_NUMERAL_AST:
		basicTy, ok := ty.(*types.Basic)
		if !ok {
			return nil, errors.Errorf("illegal type")
		}
		var u C.uint64_t
		ok = bool(C.Z3_get_numeral_uint64(s.ctx, ast, &u))
		if !ok {
			return nil, errors.Errorf("Z3_get_numeral_uint64: could not get a uint64 representation of the AST")
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
	case C.Z3_APP_AST:
		basicTy, ok := ty.(*types.Basic)
		if !ok {
			return nil, errors.Errorf("illegal type")
		}
		switch basicTy.Kind() {
		case types.String:
			s, _ := strconv.Unquote(fmt.Sprintf(`"%s"`, C.GoString(C.Z3_get_string(s.ctx, ast))))
			return s, nil
		case types.Bool:
			b := C.Z3_get_bool_value(s.ctx, ast)
			return b == C.Z3_L_TRUE, nil
		}

		return nil, errors.Errorf("cannot convert Z3_APP_AST (ast: %s) of type %s", C.GoString(C.Z3_ast_to_string(s.ctx, ast)), ty)
	}
	return nil, errors.Errorf("cannot convert Z3_AST (kind: %d) of type %s", kind, ty)
}

func sizeOfBasicKind(k types.BasicKind) uint {
	switch k {
	case types.Int:
		fallthrough
	case types.Uint:
		return strconv.IntSize
	case types.Int8:
		fallthrough
	case types.Uint8:
		return 8
	case types.Int16:
		fallthrough
	case types.Uint16:
		return 16
	case types.Int32:
		fallthrough
	case types.Uint32:
		return 32
	case types.Int64:
		fallthrough
	case types.Uint64:
		return 64
	}
	return 0
}
