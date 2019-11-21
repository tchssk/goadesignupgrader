package goadesignupgrader

import (
	"go/ast"
	"go/format"
	"os"
	"regexp"
	"strconv"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/ast/inspector"
)

var Analyzer = &analysis.Analyzer{
	Name: "goadesignupgrader",
	Doc:  Doc,
	Run:  run,
	Requires: []*analysis.Analyzer{
		inspect.Analyzer,
	},
}

const Doc = "goadesignupgrader is ..."

var regexpWildcard = regexp.MustCompile(`/:([a-zA-Z0-9_]+)`)

func run(pass *analysis.Pass) (interface{}, error) {
	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{
		(*ast.ImportSpec)(nil),
		(*ast.CallExpr)(nil),
	}
	inspect.Preorder(nodeFilter, func(n ast.Node) {
		switch n := n.(type) {
		case *ast.ImportSpec:
			path, err := strconv.Unquote(n.Path.Value)
			if err != nil {
				return
			}
			switch path {
			case "github.com/goadesign/goa/design":
				pass.Report(analysis.Diagnostic{
					Pos: n.Pos(), Message: `"github.com/goadesign/goa/design" should be removed`,
					SuggestedFixes: []analysis.SuggestedFix{{Message: "Remove", TextEdits: []analysis.TextEdit{
						{Pos: n.Pos(), End: n.End(), NewText: []byte{}},
					}}},
				})
			case "github.com/goadesign/goa/design/apidsl":
				pass.Report(analysis.Diagnostic{
					Pos: n.Path.Pos(), Message: `"github.com/goadesign/goa/design/apidsl" should be replaced with "goa.design/goa/v3/dsl"`,
					SuggestedFixes: []analysis.SuggestedFix{{Message: "Replace", TextEdits: []analysis.TextEdit{
						{Pos: n.Path.Pos(), End: n.Path.End(), NewText: []byte(`"goa.design/goa/v3/dsl"`)},
					}}},
				})
			}
		case *ast.CallExpr:
			fun, ok := n.Fun.(*ast.Ident)
			if !ok {
				return
			}
			switch fun.Name {
			case "Resource":
				pass.Report(analysis.Diagnostic{
					Pos: fun.Pos(), Message: `Resource should be replaced with Service`,
					SuggestedFixes: []analysis.SuggestedFix{{Message: "Replace", TextEdits: []analysis.TextEdit{
						{Pos: fun.Pos(), End: fun.End(), NewText: []byte("Service")},
					}}},
				})
			case "Action":
				pass.Report(analysis.Diagnostic{
					Pos: fun.Pos(), Message: `Action should be replaced with Method`,
					SuggestedFixes: []analysis.SuggestedFix{{Message: "Replace", TextEdits: []analysis.TextEdit{
						{Pos: fun.Pos(), End: fun.End(), NewText: []byte("Method")},
					}}},
				})
			case "MediaType":
				pass.Report(analysis.Diagnostic{
					Pos: fun.Pos(), Message: `MediaType should be replaced with ResultType`,
					SuggestedFixes: []analysis.SuggestedFix{{Message: "Replace", TextEdits: []analysis.TextEdit{
						{Pos: fun.Pos(), End: fun.End(), NewText: []byte("ResultType")},
					}}},
				})
			}
		}
	})

	for _, file := range pass.Files {
		astutil.Apply(file, func(c *astutil.Cursor) bool {
			switch n := c.Node().(type) {
			case *ast.CallExpr:
				fun, ok := n.Fun.(*ast.Ident)
				if !ok {
					return true
				}
				switch fun.Name {
				case "GET", " HEAD", " POST", " PUT", " DELETE", " CONNECT", " OPTIONS", " TRACE", " PATCH":
					// Replace colons with curly braces in HTTP routing DSLs.
					for _, arg := range n.Args {
						b := arg.(*ast.BasicLit)
						b.Value = replaceWildcard(b.Value)
					}
				}
			case *ast.Ident:
				switch n.Name {
				case "Integer":
					// Replace Integer with Int.
					n.Name = "Int"
				case "DateTime":
					// Replace DateTime with String + Format(FormatDateTime).
					n.Name = "String"
					switch nn := c.Parent().(type) {
					case *ast.CallExpr:
						fun, ok := nn.Args[len(nn.Args)-1].(*ast.FuncLit)
						if !ok {
							fun = &ast.FuncLit{
								Type: &ast.FuncType{},
								Body: &ast.BlockStmt{},
							}
							nn.Args = append(nn.Args, fun)
						}
						fun.Body.List = append(fun.Body.List, &ast.ExprStmt{
							X: &ast.CallExpr{
								Fun: &ast.Ident{
									Name: "Format",
								},
								Args: []ast.Expr{
									&ast.Ident{
										Name: "FormatDateTime",
									},
								},
							},
						})
					}
				}
			case *ast.ExprStmt:
				cal, ok := n.X.(*ast.CallExpr)
				if !ok {
					return true
				}
				fun, ok := cal.Fun.(*ast.Ident)
				if !ok {
					return true
				}
				switch fun.Name {
				case "BasePath":
					// Replace BasePath with Path and move it into HTTP.
					fun.Name = "Path"
					switch nn := c.Parent().(type) {
					case *ast.BlockStmt:
						var (
							index int
							http  *ast.CallExpr
						)
						for i, v := range nn.List {
							switch nnn := v.(type) {
							case *ast.ExprStmt:
								call, ok := nnn.X.(*ast.CallExpr)
								if !ok {
									continue
								}
								funn, ok := call.Fun.(*ast.Ident)
								if !ok {
									continue
								}
								switch funn.Name {
								case "HTTP":
									http = call
								case "BasePath":
									index = i
								}
							}
						}
						if http == nil {
							http = &ast.CallExpr{
								Fun: &ast.Ident{
									Name: "HTTP",
								},
								Args: []ast.Expr{},
							}
							nn.List = append([]ast.Stmt{
								&ast.ExprStmt{
									X: http,
								},
							}, nn.List...)
							index++
						}
						var (
							ok   bool
							funn *ast.FuncLit
						)
						if len(http.Args) > 0 {
							funn, ok = http.Args[len(http.Args)-1].(*ast.FuncLit)
						}
						if !ok {
							funn = &ast.FuncLit{
								Type: &ast.FuncType{},
								Body: &ast.BlockStmt{},
							}
							http.Args = append(http.Args, funn)
						}
						funn.Body.List = append(funn.Body.List, n)
						nn.List = append(nn.List[:index], nn.List[index+1:]...)
					}
				}
			}
			return true
		}, nil)

		f, err := os.OpenFile(pass.Fset.File(file.Pos()).Name(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		if err := format.Node(f, pass.Fset, file); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func replaceWildcard(s string) string {
	return regexpWildcard.ReplaceAllString(s, "/{$1}")
}
