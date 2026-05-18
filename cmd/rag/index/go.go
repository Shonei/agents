package index

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

type GoStrategy struct{}

func NewGoStrategy() *GoStrategy {
	return &GoStrategy{}
}

func (s *GoStrategy) Chunk(path string, content string) ([]Chunk, error) {
	if !strings.HasSuffix(path, ".go") {
		return []Chunk{{Content: content, Meta: map[string]string{}}}, nil
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, content, parser.ParseComments)
	if err != nil {
		// Fallback to whole file
		return []Chunk{{Content: content, Meta: map[string]string{}}}, nil
	}

	var chunks []Chunk

	// Extract package docs
	if f.Doc != nil {
		chunks = append(chunks, Chunk{
			Content: f.Doc.Text(),
			Meta: map[string]string{
				"symbol_type": "package_doc",
				"symbol_name": f.Name.Name,
			},
		})
	}

	// Extract declarations
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			if d.Tok == token.TYPE {
				start := fset.Position(d.Pos()).Offset
				end := fset.Position(d.End()).Offset
				if start >= 0 && end <= len(content) && start < end {
					// Find the primary name and type
					var name string
					symType := "type"

					if len(d.Specs) > 0 {
						if ts, ok := d.Specs[0].(*ast.TypeSpec); ok {
							name = ts.Name.Name
							if _, isStruct := ts.Type.(*ast.StructType); isStruct {
								symType = "struct"
							} else if _, isInterface := ts.Type.(*ast.InterfaceType); isInterface {
								symType = "interface"
							}
						}
					}

					chunks = append(chunks, Chunk{
						Content: content[start:end],
						Meta: map[string]string{
							"symbol_type": symType,
							"symbol_name": name,
						},
					})
				}
			}
		case *ast.FuncDecl:
			start := fset.Position(d.Pos()).Offset
			end := fset.Position(d.End()).Offset
			if start >= 0 && end <= len(content) && start < end {
				name := d.Name.Name
				if d.Recv != nil && len(d.Recv.List) > 0 {
					// It's a method, try to get receiver type
					switch t := d.Recv.List[0].Type.(type) {
					case *ast.Ident:
						name = t.Name + "." + name
					case *ast.StarExpr:
						if ident, ok := t.X.(*ast.Ident); ok {
							name = "*" + ident.Name + "." + name
						}
					}
				}
				chunks = append(chunks, Chunk{
					Content: content[start:end],
					Meta: map[string]string{
						"symbol_type": "function",
						"symbol_name": name,
					},
				})
			}
		}
	}

	if len(chunks) == 0 {
		return []Chunk{{Content: content, Meta: map[string]string{}}}, nil
	}

	return chunks, nil
}
