package congo

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
)

func type2ASTExpr(ty types.Type) ast.Expr {
	switch ty := ty.(type) {
	case *types.Basic:
		return ast.NewIdent(ty.Name())
	case *types.Named:
		return &ast.SelectorExpr{
			X:   ast.NewIdent(ty.Obj().Pkg().Name()),
			Sel: ast.NewIdent(ty.Obj().Id()),
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
		case info&types.IsInteger > 0:
			return &ast.BasicLit{
				Kind:  token.INT,
				Value: fmt.Sprintf("%v", v),
			}
		default:
			panic("unimplemented")
		}
	}
	panic("unimplemented")
}
