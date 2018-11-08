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

type z3Datatype interface {
	Sort() C.Z3_sort
}

type z3DatatypeDict map[string]z3Datatype

type z3PointerDatatype struct {
	nilDecl C.Z3_func_decl
	refDecl C.Z3_func_decl
	refAcc  C.Z3_func_decl
	isNil   C.Z3_func_decl
	isRef   C.Z3_func_decl
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

func newSort(ctx C.Z3_context, ty types.Type, name string, datatypes z3DatatypeDict) C.Z3_sort {
	switch ty := ty.(type) {
	case *types.Basic:
		return newBasicSort(ctx, ty)
	case *types.Pointer:
		dt := getPointerDatatype(ctx, ty, datatypes)
		return dt.sort
	case *types.Named:
		return newSort(ctx, ty.Underlying(), ty.String(), datatypes)
	case *types.Struct:
		dt := getStructDatatype(ctx, ty, name, datatypes)
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

// getPointerDatatype returns a datatype for pointers and
// creates a new one if not registered.
// datatype pointer a = nil | ref a
func getPointerDatatype(ctx C.Z3_context, ty *types.Pointer, datatypes z3DatatypeDict) *z3PointerDatatype {
	elemTy := ty.Elem()
	if datatype, ok := datatypes[ty.String()]; ok {
		return datatype.(*z3PointerDatatype)
	}

	nilConsSymbol := z3MkStringSymbol(ctx, "nil")
	nilRecogSymbol := z3MkStringSymbol(ctx, "is-nil")
	nilCons := C.Z3_mk_constructor(ctx, nilConsSymbol, nilRecogSymbol, 0, nil, nil, nil)
	defer C.Z3_del_constructor(ctx, nilCons)

	refConsSymbol := z3MkStringSymbol(ctx, "ref")
	refRecogSymbol := z3MkStringSymbol(ctx, "is-ref")
	refFieldSymbol := z3MkStringSymbol(ctx, "deref")
	refSort := newSort(ctx, elemTy, "", datatypes)
	refSortRef := C.uint(0)
	refCons := C.Z3_mk_constructor(ctx, refConsSymbol, refRecogSymbol, 1, &refFieldSymbol, &refSort, &refSortRef)
	defer C.Z3_del_constructor(ctx, refCons)

	constructors := [...]C.Z3_constructor{nilCons, refCons}
	name := fmt.Sprintf("ref-%s", elemTy.String())
	datatypeSymbol := z3MkStringSymbol(ctx, name)
	sort := C.Z3_mk_datatype(ctx, datatypeSymbol, 2, &constructors[0])

	var nilDecl, isNil, refDecl, isRef, refAcc C.Z3_func_decl
	C.Z3_query_constructor(ctx, nilCons, 0, &nilDecl, &isNil, nil)
	C.Z3_query_constructor(ctx, refCons, 1, &refDecl, &isRef, &refAcc)

	datatype := &z3PointerDatatype{
		nilDecl: nilDecl,
		refDecl: refDecl,
		refAcc:  refAcc,
		isNil:   isNil,
		isRef:   isRef,
		sort:    sort,
	}

	datatypes[ty.String()] = datatype
	return datatype
}

// getStructDatatype returns a datatype for structs and
// creates a new one if not registered.
// TODO(ajalab) zero field struct
func getStructDatatype(ctx C.Z3_context, ty *types.Struct, name string, datatypes z3DatatypeDict) *z3StructDatatype {
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
