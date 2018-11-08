package congo

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"log"
	"strconv"
	"unsafe"

	"golang.org/x/tools/go/ssa"
)

func zero(ty types.Type) interface{} {
	switch t := ty.(type) {
	case *types.Basic:
		if t.Kind() == types.UntypedNil {
			panic("untyped nil has no zero value")
		}
		switch t.Kind() {
		case types.Bool:
			return false
		case types.Int:
			return int(0)
		case types.Int8:
			return int8(0)
		case types.Int16:
			return int16(0)
		case types.Int32:
			return int32(0)
		case types.Int64:
			return int64(0)
		case types.Uint:
			return uint(0)
		case types.Uint8:
			return uint8(0)
		case types.Uint16:
			return uint16(0)
		case types.Uint32:
			return uint32(0)
		case types.Uint64:
			return uint64(0)
		case types.Uintptr:
			return uintptr(0)
		case types.Float32:
			return float32(0)
		case types.Float64:
			return float64(0)
		case types.Complex64:
			return complex64(0)
		case types.Complex128:
			return complex128(0)
		case types.String:
			return ""
		case types.UnsafePointer:
			return unsafe.Pointer(nil)
		default:
			panic(fmt.Sprint("zero for unexpected type:", t))
		}
	case *types.Pointer:
		return (*interface{})(nil)
	case *types.Array:
		a := make([]interface{}, t.Len())
		for i := range a {
			a[i] = zero(t.Elem())
		}
		return a
	case *types.Named:
		return zero(t.Underlying())
	case *types.Interface:
		panic("unimplemented")
	case *types.Slice:
		return []interface{}(nil)
	case *types.Struct:
		s := make([]interface{}, t.NumFields())
		for i := range s {
			s[i] = zero(t.Field(i).Type())
		}
		return s
	case *types.Tuple:
		if t.Len() == 1 {
			return zero(t.At(0).Type())
		}
		s := make([]interface{}, t.Len())
		for i := range s {
			s[i] = zero(t.At(i).Type())
		}
		return s
	case *types.Chan:
		return chan interface{}(nil)
	case *types.Map:
		return map[interface{}][]interface{}(nil)
	case *types.Signature:
		return (*ssa.Function)(nil)
	}

	panic(fmt.Sprint("zero: unexpected ", ty))
}

func type2ASTExpr(ty types.Type) ast.Expr {
	switch ty := ty.(type) {
	case *types.Basic:
		return ast.NewIdent(ty.Name())
	case *types.Named:
		return &ast.SelectorExpr{
			X:   ast.NewIdent(ty.Obj().Pkg().Name()),
			Sel: ast.NewIdent(ty.Obj().Id()),
		}
	case *types.Pointer:
		return &ast.StarExpr{
			X: type2ASTExpr(ty.Elem()),
		}
	default:
		panic("unimplemented")
	}
}

func value2ASTExpr(v interface{}, ty types.Type) ast.Expr {
	switch ty := ty.(type) {
	case *types.Basic:
		info := ty.Info()
		switch {
		case info&types.IsBoolean > 0:
			s := "false"
			if v.(bool) {
				s = "true"
			}
			return ast.NewIdent(s)
		case info&types.IsInteger > 0:
			return &ast.BasicLit{
				Kind:  token.INT,
				Value: fmt.Sprintf("%v", v),
			}
		case info&types.IsString > 0:
			return &ast.BasicLit{
				Kind:  token.STRING,
				Value: strconv.Quote(v.(string)),
			}
		default:
			panic("unimplemented")
		}
	case *types.Pointer:
		p := v.(*interface{})
		if p == nil {
			return ast.NewIdent("nil")
		}
		if basicTy, ok := ty.Elem().(*types.Basic); ok {
			return &ast.CallExpr{
				Fun: ast.NewIdent(basicTy.Name() + "ptr"),
				Args: []ast.Expr{
					value2ASTExpr(*p, basicTy),
				},
			}
		}

		namedTy, ok := ty.Elem().(*types.Named)
		if !ok {
			log.Fatalf("pointer of unnamed and non-basic type is not supported")
			panic("unimplemented")
		}
		typeName := namedTy.Obj()
		name := typeName.Name()
		pkgName := typeName.Pkg().Name()
		elem := ty.Elem().Underlying()
		switch ty := elem.(type) {
		case *types.Struct:
			n := ty.NumFields()
			elts := make([]ast.Expr, 0, n)
			for i := 0; i < n; i++ {
				e := ty.Field(i)
				elts = append(elts, &ast.KeyValueExpr{
					Key:   ast.NewIdent(e.Name()),
					Value: value2ASTExpr(((*p).([]interface{}))[i], e.Type()),
				})
			}
			return &ast.UnaryExpr{
				Op: token.AND,
				X: &ast.CompositeLit{
					Type: &ast.SelectorExpr{
						X:   ast.NewIdent(pkgName),
						Sel: ast.NewIdent(name),
					},
					Elts: elts,
				},
			}
		}
		panic("unimplemented")
	}
	panic("unimplemented")
}
