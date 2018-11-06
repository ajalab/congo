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

type z3StructDatatype struct {
	n         int
	is        C.Z3_func_decl
	decl      C.Z3_func_decl
	accessors []C.Z3_func_decl
	sort      C.Z3_sort
}

func (dt *z3StructDatatype) Sort() C.Z3_sort {
	return dt.sort
}

func z3MkStringSymbol(ctx C.Z3_context, s string) C.Z3_symbol {
	c := C.CString(s)
	defer C.free(unsafe.Pointer(c))
	return C.Z3_mk_string_symbol(ctx, c)
}

func newSort(ctx C.Z3_context, ty types.Type, name string, datatypes z3DatatypeDict) C.Z3_sort {
	switch ty := ty.(type) {
	case *types.Basic:
		return newBasicSort(ctx, ty)
	case *types.Pointer:
		dt := newPointerDatatype(ctx, ty, datatypes)
		return dt.sort
	case *types.Named:
		return newSort(ctx, ty.Underlying(), ty.String(), datatypes)
	case *types.Struct:
		dt := newStructDatatype(ctx, ty, name, datatypes)
		return dt.sort
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
func newPointerDatatype(ctx C.Z3_context, ty *types.Pointer, datatypes z3DatatypeDict) *z3PointerDatatype {
	elemTy := ty.Elem()
	name := fmt.Sprintf("ref-%s", elemTy.String())
	if datatype, ok := datatypes[ty.String()]; ok {
		return datatype.(*z3PointerDatatype)
	}
	valSort := newSort(ctx, elemTy, "", datatypes)

	z3NilSymbolName := C.CString("nil")
	z3NilRecogSymbolName := C.CString("is-nil")
	z3NilSymbol := C.Z3_mk_string_symbol(ctx, z3NilSymbolName)
	z3NilRecogSymbol := C.Z3_mk_string_symbol(ctx, z3NilRecogSymbolName)
	z3NilCons := C.Z3_mk_constructor(ctx, z3NilSymbol, z3NilRecogSymbol, 0, nil, nil, nil)
	z3ValSymbolName := C.CString("ref")
	z3ValRecogSymbolName := C.CString("is-ref")

	z3ValSymbol := C.Z3_mk_string_symbol(ctx, z3ValSymbolName)
	z3ValRecogSymbol := C.Z3_mk_string_symbol(ctx, z3ValRecogSymbolName)
	z3ValFieldSymbol := z3MkStringSymbol(ctx, "deref")
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
	return datatype
}

// TODO(ajalab) zero field struct
func newStructDatatype(ctx C.Z3_context, ty *types.Struct, name string, datatypes z3DatatypeDict) *z3StructDatatype {
	if datatype, ok := datatypes[ty.String()]; ok {
		return datatype.(*z3StructDatatype)
	}

	n := ty.NumFields()
	symbols := make([]C.Z3_symbol, n)
	sorts := make([]C.Z3_sort, n)
	sortRefs := make([]C.uint, n)
	for i := 0; i < n; i++ {
		field := ty.Field(i)
		sorts[i] = newSort(ctx, field.Type(), "", datatypes)
		symbols[i] = z3MkStringSymbol(ctx, field.Name())
	}
	constructorSymbol := z3MkStringSymbol(ctx, name)
	recognizerSymbol := z3MkStringSymbol(ctx, "is-"+name)
	constructor := C.Z3_mk_constructor(
		ctx, constructorSymbol, recognizerSymbol, C.uint(n), &symbols[0], &sorts[0], &sortRefs[0],
	)
	if name == "" {
		name = fmt.Sprintf("unnamed-struct-%d", len(datatypes))
	}
	datatypeSymbol := z3MkStringSymbol(ctx, name)
	sort := C.Z3_mk_datatype(ctx, datatypeSymbol, 1, &constructor)

	var decl, is C.Z3_func_decl
	accessors := make([]C.Z3_func_decl, n)
	C.Z3_query_constructor(ctx, constructor, C.uint(n), &decl, &is, &accessors[0])
	C.Z3_del_constructor(ctx, constructor)

	datatype := &z3StructDatatype{
		n:         n,
		is:        is,
		decl:      decl,
		accessors: accessors,
		sort:      sort,
	}

	datatypes[ty.String()] = datatype
	return datatype
}
