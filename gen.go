package congo

import (
	"go/ast"
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
