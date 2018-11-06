package solver

import (
	"fmt"
	"go/types"
	"log"

	/*
		#cgo LDFLAGS: -lz3
		#include <stdlib.h>
		#include <z3.h>
	*/
	"C"
)
import "unsafe"

type z3Datatype interface {
	Sort() C.Z3_sort
}

type z3DatatypeDict map[string]z3Datatype

type z3PointerDatatype struct {
	nilDecl C.Z3_func_decl
	valDecl C.Z3_func_decl
	valAcc  C.Z3_func_decl
	isNil   C.Z3_func_decl
	isVal   C.Z3_func_decl
	sort    C.Z3_sort
}

func (dt *z3PointerDatatype) Sort() C.Z3_sort {
	return dt.sort
}

func newSort(ctx C.Z3_context, ty types.Type, datatypes z3DatatypeDict) C.Z3_sort {
	switch ty := ty.(type) {
	case *types.Basic:
		return newBasicSort(ctx, ty)
	case *types.Pointer:
		return newPointerSort(ctx, ty, datatypes)
	}
	log.Fatalf("unsupported type: %[1]v: %[1]T", ty)
	panic("unimplemented")
}

func newBasicSort(ctx C.Z3_context, ty *types.Basic) C.Z3_sort {
	info := ty.Info()
	switch {
	case info&types.IsBoolean > 0:
		return C.Z3_mk_bool_sort(ctx)
	case info&types.IsInteger > 0:
		return C.Z3_mk_bv_sort(ctx, C.uint(sizeOfBasicKind(ty.Kind())))
	case info&types.IsString > 0:
		return C.Z3_mk_string_sort(ctx)
	}

	log.Fatalf("unsupported basic type: %v: %v", ty, ty.Kind())
	panic("unimplemented")
}

// newPointerDatatype creates a new datatype for pointers.
// datatype pointer a = nil | val a
func newPointerSort(ctx C.Z3_context, ty *types.Pointer, datatypes z3DatatypeDict) C.Z3_sort {
	elemTy := ty.Elem()
	name := fmt.Sprintf("p-%s", elemTy.String())
	if datatype, ok := datatypes[ty.String()]; ok {
		return datatype.Sort()
	}
	valSort := newSort(ctx, elemTy, datatypes)

	z3NilSymbolName := C.CString("nil")
	z3NilRecogSymbolName := C.CString("is-nil")
	z3NilSymbol := C.Z3_mk_string_symbol(ctx, z3NilSymbolName)
	z3NilRecogSymbol := C.Z3_mk_string_symbol(ctx, z3NilRecogSymbolName)
	z3NilCons := C.Z3_mk_constructor(ctx, z3NilSymbol, z3NilRecogSymbol, 0, nil, nil, nil)
	z3ValSymbolName := C.CString("val")
	z3ValRecogSymbolName := C.CString("is-val")

	z3ValSymbol := C.Z3_mk_string_symbol(ctx, z3ValSymbolName)
	z3ValRecogSymbol := C.Z3_mk_string_symbol(ctx, z3ValRecogSymbolName)
	z3ValFieldSymbol := C.Z3_mk_int_symbol(ctx, 0)
	z3ValSortRef := C.uint(0)
	z3ValCons := C.Z3_mk_constructor(ctx, z3ValSymbol, z3ValRecogSymbol, 1, &z3ValFieldSymbol, &valSort, &z3ValSortRef)
	z3Constructors := [...]C.Z3_constructor{z3NilCons, z3ValCons}
	z3DatatypeSymbolName := C.CString(name)
	z3DatatypeSymbol := C.Z3_mk_string_symbol(ctx, z3DatatypeSymbolName)
	sort := C.Z3_mk_datatype(ctx, z3DatatypeSymbol, 2, &z3Constructors[0])

	var nilDecl, isNil, valDecl, isVal, valAcc C.Z3_func_decl
	C.Z3_query_constructor(ctx, z3NilCons, 0, &nilDecl, &isNil, nil)
	C.Z3_query_constructor(ctx, z3ValCons, 1, &valDecl, &isVal, &valAcc)

	datatype := &z3PointerDatatype{
		nilDecl: nilDecl,
		valDecl: valDecl,
		valAcc:  valAcc,
		isNil:   isNil,
		isVal:   isVal,
		sort:    sort,
	}

	C.Z3_del_constructor(ctx, z3NilCons)
	C.Z3_del_constructor(ctx, z3ValCons)
	C.free(unsafe.Pointer(z3NilSymbolName))
	C.free(unsafe.Pointer(z3NilRecogSymbolName))
	C.free(unsafe.Pointer(z3ValSymbolName))
	C.free(unsafe.Pointer(z3ValRecogSymbolName))
	C.free(unsafe.Pointer(z3DatatypeSymbolName))

	datatypes[ty.String()] = datatype
	return datatype.sort
}

/*
func newStructSort(ctx C.Z3_context, ty *types.Struct, datatypes z3DatatypeDict) {
	n := ty.NumFields()
	for i := 0; i < n; i++ {
		field := ty.Field(i)
	}
}
*/
