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
	"os"
	"strconv"
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/tools/go/ssa"

	"github.com/ajalab/congo/trace"
)

const (
	z3SymbolPrefixForSymbol string = "symbol-"
)

func z3MkStringSymbol(ctx C.Z3_context, s string) C.Z3_symbol {
	c := C.CString(s)
	defer C.free(unsafe.Pointer(c))
	return C.Z3_mk_string_symbol(ctx, c)
}

// Z3Solver is a type that holds the Z3 context, assertions, and symbols.
type Z3Solver struct {
	asts     map[ssa.Value]C.Z3_ast
	refs     map[ssa.Value]ssa.Value
	ctx      C.Z3_context
	branches []Branch
	symbols  []ssa.Value
}

//export goZ3ErrorHandler
func goZ3ErrorHandler(ctx C.Z3_context, e C.Z3_error_code) {
	msg := C.Z3_get_error_msg(ctx, e)
	panic("Z3 error occurred: " + C.GoString(msg))
}

// CreateZ3Solver returns a new Z3Solver.
func CreateZ3Solver(symbols []ssa.Value, trace *trace.Trace) (*Z3Solver, error) {
	cfg := C.Z3_mk_config()
	defer C.Z3_del_config(cfg)

	// TODO(ajalab): We may have to use Z3_mk_context_rc and manually handle the reference count.
	ctx := C.Z3_mk_context(cfg)
	C.Z3_set_error_handler(ctx, (*C.Z3_error_handler)(C.goZ3ErrorHandler))

	s := &Z3Solver{
		asts: make(map[ssa.Value]C.Z3_ast),
		refs: make(map[ssa.Value]ssa.Value),
		ctx:  ctx,
	}

	err := s.loadSymbols(symbols)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load symbols")
	}
	s.loadTrace(trace)

	return s, nil
}

// Close deletes the Z3 context.
func (s *Z3Solver) Close() {
	C.Z3_del_context(s.ctx)
}

/*
func (s *Z3Solver) getSymbolAST(name string, ty types.Type) C.Z3_ast {
	z3Symbol := z3MkStringSymbol(s.ctx, name)
	switch ty := ty.(type) {
	case *types.Basic:
		sort := newBasicSort(s.ctx, ty)
		return C.Z3_mk_const(s.ctx, z3Symbol, sort)
	case *types.Pointer:

	}
	log.Fatalf("unsupported type: %[1]v: %[1]T", ty)
	panic("unimplemented")
}
*/

func (s *Z3Solver) loadSymbol(symbol ssa.Value, name string) {
	ty := symbol.Type()
	z3Symbol := z3MkStringSymbol(s.ctx, name)
	switch ty := ty.(type) {
	case *types.Basic:
		sort := newBasicSort(s.ctx, ty)
		ast := C.Z3_mk_const(s.ctx, z3Symbol, sort)
		s.asts[symbol] = ast
	case *types.Pointer:
		// ast represents a pointer value
		sort := C.Z3_mk_int_sort(s.ctx)
		ast := C.Z3_mk_const(s.ctx, z3Symbol, sort)
		s.asts[symbol] = ast

		ref := &Ref{symbol}
		s.refs[symbol] = ref
		s.loadSymbol(ref, "*"+name)
	}
}

// loadSymbols loads symbolic variables to the solver.
func (s *Z3Solver) loadSymbols(symbols []ssa.Value) error {
	s.symbols = make([]ssa.Value, len(symbols))
	for i, symbol := range symbols {
		// TODO(ajalab): rename
		name := fmt.Sprintf("%s%d", z3SymbolPrefixForSymbol, i)
		s.loadSymbol(symbol, name)
	}
	copy(s.symbols, symbols)
	return nil
}

// loadTrace loads a running trace to the solver.
func (s *Z3Solver) loadTrace(tr *trace.Trace) {
	var currentBlock *ssa.BasicBlock
	var prevBlock *ssa.BasicBlock
	var callStack []*ssa.Call

	instrs := tr.Instrs()
	isComplete := tr.IsComplete()

	// If the trace is not complete, ignore the last instruction,
	// which is a cause of failure.
	n := len(instrs)
	if !isComplete {
		n = n - 1
	}

	for i := 0; i < n; i++ {
		instr := instrs[i]
		block := instr.Block()
		if currentBlock != block {
			prevBlock = currentBlock
			currentBlock = block
			log.Printf("block: %v.%s", block.Parent(), block)
		}

		switch instr := instr.(type) {
		case *ssa.UnOp:
			var err error
			s.asts[instr], err = s.unop(instr)
			if err != nil {
				log.Println(err)
			}
		case *ssa.BinOp:
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
				if i < len(instrs)-1 && instrs[i+1].Parent() == fn {
					for j, arg := range instr.Call.Args {
						log.Printf("call %v param%d %v <- %v", fn, j, fn.Params[j], arg)
						s.asts[fn.Params[j]] = s.get(arg)
					}
					callStack = append(callStack, instr)
				} else {
					log.Printf("ignored function call %v (trace[i + 1]) = %v", instr, instrs[i+1])
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
			// len(callStack) becomes 0 when instr.Parent() is init() or main() of
			// the runner package.
			if len(callStack) > 0 {
				callInstr := callStack[len(callStack)-1]
				// TODO(ajalab) Support multiple return values
				switch len(instr.Results) {
				case 0:
				case 1:
					s.asts[callInstr] = s.get(instr.Results[0])
				default:
					log.Println("multiple return values are not supported")
				}
				callStack = callStack[:len(callStack)-1]
			}
		case *ssa.FieldAddr:
			// &((*pstruct).field)
			/*
				k := instr.Field
				tyPStruct := instr.X.Type().Underlying().(*types.Pointer) //.Elem()
				tyElem := tyPStruct.Elem()
				tyStruct := tyElem.Underlying().(*types.Struct)
				tyPField := instr.Type().(*types.Pointer)
				pointerStructDT := getPointerDatatype(s.ctx, tyPStruct, s.datatypes)
				pointerFieldDT := getPointerDatatype(s.ctx, tyPField, s.datatypes)
				structDT := getStructDatatype(s.ctx, tyStruct, tyElem.String(), s.datatypes)
				ast := s.get(instr.X)
				deref := C.Z3_mk_app(s.ctx, pointerStructDT.refAcc, 1, &ast)
				acc := C.Z3_mk_app(s.ctx, structDT.accessors[k], 1, &deref)
				ref := C.Z3_mk_app(s.ctx, pointerFieldDT.refDecl, 1, &acc)
				s.nonNil[instr.X] = struct{}{}
				s.asts[instr] = ref
			*/
		}
		if ifInstr, ok := instr.(*ssa.If); ok {
			if s.get(ifInstr.Cond) != nil {
				thenBlock := instr.Block().Succs[0]
				nextBlock := instrs[i+1].Block()
				s.branches = append(s.branches, &BranchIf{
					instr:     ifInstr,
					direction: thenBlock == nextBlock,
				})
			}
		}
	}
	// Execution was stopped due to panic
	if !isComplete {
		causeInstr := instrs[len(instrs)-1]
		switch instr := causeInstr.(type) {
		case *ssa.UnOp:
			s.branches = append(s.branches, &BranchDeref{
				instr:   instr,
				success: false,
				x:       instr.X,
			})
		case *ssa.FieldAddr:
			s.branches = append(s.branches, &BranchDeref{
				instr:   instr,
				success: false,
				x:       instr.X,
			})
		default:
			log.Fatalf("panic caused by %[1]v: %[1]T but not supported", instr)
			panic("unreachable")
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
	case token.MUL:
		// TODO(ajalab): existance check
		ref := s.refs[instr.X]
		return s.get(ref), nil
		/*
			pointer := instr.X
			pointerTy := pointer.Type().(*types.Pointer)
			pointerAST := s.get(pointer)
			if pointerAST == nil {
				return nil, errors.Errorf("corresponding AST for pointer was not found: %v", pointer)
			}
			pointerDT := getPointerDatatype(s.ctx, pointerTy, s.datatypes)
			ref := C.Z3_mk_app(s.ctx, pointerDT.refAcc, 1, &pointerAST)
			s.nonNil[pointer] = struct{}{}
			return ref, nil
		*/
		// case token.XOR:
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
	case *types.Pointer:
		if v.Value == nil {
			sort := C.Z3_mk_int_sort(s.ctx)
			return C.Z3_mk_unsigned_int(s.ctx, C.uint(0), sort)
		}
	}
	log.Fatalf("getConstAST: Unimplemented const value %v: %T", v, v.Type())
	panic("unimplemented")
}

/*
func (s *Z3Solver) hasBranchAST(branch Branch) bool {
	switch b := branch.(type) {
	case *BranchIf:
		return s.get(b.Cond()) != nil
	case *PanicNilPointerDeref:
		return s.get(b.X()) != nil
	default:
		panic("unimplemented")
	}
}
*/

func (s *Z3Solver) Branches() []Branch {
	return s.branches
}

func (s *Z3Solver) getBranchAST(branch Branch, negate bool) (C.Z3_ast, error) {
	switch b := branch.(type) {
	case *BranchIf:
		cond := s.get(b.instr.Cond)
		if cond == nil {
			return nil, errors.Errorf("corresponding AST for branching condition was not found: %+v in %v",
				branch.Instr(),
				b.Instr().Parent(),
			)
		}
		if (!negate && !b.direction) || (negate && b.direction) {
			cond = C.Z3_mk_not(s.ctx, cond)
		}
		return cond, nil
	case *BranchDeref:
		pointer := s.get(b.x)
		if pointer == nil {
			return nil, errors.Errorf("corresponding AST for pointer dereference was not found: %+v", b.x)
		}
		sort := C.Z3_mk_int_sort(s.ctx)
		zero := C.Z3_mk_unsigned_int(s.ctx, C.uint(0), sort)
		cond := C.Z3_mk_eq(s.ctx, pointer, zero)
		if b.success {
			cond = C.Z3_mk_not(s.ctx, cond)
		}
		return cond, nil

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
		cond, err := s.getBranchAST(branch, false)
		if err != nil {
			return nil, errors.Wrap(err, "failed to solve constraints")
		}
		C.Z3_solver_assert(s.ctx, solver, cond)
	}

	negBranch := s.branches[negate]
	negCond, err := s.getBranchAST(negBranch, true)
	if err != nil {
		return nil, errors.Wrap(err, "failed to solve constraints")
	}
	C.Z3_solver_assert(s.ctx, solver, negCond)
	/*
		for v := range s.nonNil {
			ast := s.get(v)
			pointerDT := getPointerDatatype(s.ctx, v.Type().(*types.Pointer), s.datatypes)
			C.Z3_solver_assert(s.ctx, solver, C.Z3_mk_app(s.ctx, pointerDT.isRef, 1, &ast))
		}
	*/

	fmt.Fprintf(os.Stderr, "solver\n%s\n", C.GoString(C.Z3_solver_to_string(s.ctx, solver)))

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
		fmt.Fprintf(os.Stderr, "model\n%s\n", C.GoString(C.Z3_model_to_string(s.ctx, m)))
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
	for i, symbol := range s.symbols {
		var err error
		values[i], err = s.getValueFromModel(m, symbol)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get a value for the symbol[%d]", i)
		}
	}
	return values, nil
}

func (s *Z3Solver) getASTFromModel(m C.Z3_model, v ssa.Value) (C.Z3_ast, error) {
	var result C.Z3_ast
	ast := s.asts[v]
	ok := C.Z3_model_eval(s.ctx, m, ast, C.bool(true), &result)
	if !C.bool(ok) {
		return nil, errors.Errorf("failed to extract a concrete AST for %v from the model", v)
	}
	return result, nil
}

func (s *Z3Solver) getValueFromModel(m C.Z3_model, v ssa.Value) (interface{}, error) {
	ast, err := s.getASTFromModel(m, v)
	if err != nil {
		return nil, err
	}
	ty := v.Type().Underlying()
	switch ty := ty.(type) {
	case *types.Basic:
		info := ty.Info()
		switch {
		case info&types.IsInteger > 0:
			var u C.uint64_t
			if ok := bool(C.Z3_get_numeral_uint64(s.ctx, ast, &u)); !ok {
				return nil, errors.Errorf("Z3_get_numeral_uint64: could not get an uint64 representation of the AST")
			}
			switch ty.Kind() {
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
		case info&types.IsBoolean > 0:
			b := C.Z3_get_bool_value(s.ctx, ast)
			return b == C.Z3_L_TRUE, nil
		case info&types.IsString > 0:
			s, _ := strconv.Unquote(fmt.Sprintf(`"%s"`, C.GoString(C.Z3_get_string(s.ctx, ast))))
			return s, nil
		}
	case *types.Pointer:
		var i C.int
		if ok := bool(C.Z3_get_numeral_int(s.ctx, ast, &i)); !ok {
			return nil, errors.Errorf("Z3_get_numeral_int: could not get a uint64 representation of the AST")
		}
		pointerRegion := int(i)
		switch pointerRegion {
		case 0:
			return nil, nil
		case 1:
			ref := s.refs[v]
			v, err := s.getValueFromModel(m, ref)
			return &v, err
		}
	}

	return nil, errors.Errorf("type %v is not supported", ty)
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

// UnsatError is an error describing that Z3 constraints were unsatisfied.
type UnsatError struct{}

func (ue UnsatError) Error() string {
	return "unsat"
}
