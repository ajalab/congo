package solver

import (
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

// newPointerDatatype creates a new datatype for pointers.
// datatype pointer a = nil | val a
func newPointerSort(ctx C.Z3_context, valSort C.Z3_sort, name string) *z3PointerDatatype {
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

	return datatype
}
