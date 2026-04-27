// Package coverage_test (godoc) enforces Constitution Principle VI:
// every exported identifier in the shipped packages (parse, model,
// render, findings) has a non-empty doc comment.
//
// CI invokes this test directly via
// `go test ./tests/coverage/... -run TestGodocCoverage` and the
// pre-push constitution guard runs the same target locally so a
// missing godoc fails before the push reaches origin (Gate 5,
// MECHANICAL as of constitution v1.4).
package coverage_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	"github.com/matias-sanchez/My-gather/tests/goldens"
)

var godocCheckedPackages = []string{"parse", "model", "render", "findings"}

// TestGodocCoverage is the canonical entry point invoked by CI and
// the pre-push hook. It delegates to the table-driven sub-tests in
// TestEveryExportedIdentifierIsDocumented so the existing assertion
// logic stays in one place.
func TestGodocCoverage(t *testing.T) {
	TestEveryExportedIdentifierIsDocumented(t)
}

func TestEveryExportedIdentifierIsDocumented(t *testing.T) {
	root := goldens.RepoRoot(t)
	fset := token.NewFileSet()

	for _, pkg := range godocCheckedPackages {
		pkg := pkg
		t.Run(pkg, func(t *testing.T) {
			dir := filepath.Join(root, pkg)
			_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
					return nil
				}
				file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
				if err != nil {
					t.Fatalf("parse %s: %v", path, err)
				}
				checkFile(t, path, file)
				return nil
			})
		})
	}
}

func checkFile(t *testing.T, path string, file *ast.File) {
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Recv != nil {
				// methods: require a doc comment if the receiver type is
				// exported AND the method is exported.
				if isExported(d.Name.Name) && receiverIsExported(d.Recv) {
					if d.Doc == nil || strings.TrimSpace(d.Doc.Text()) == "" {
						t.Errorf("%s: method %q has no godoc comment",
							short(path), d.Name.Name)
					}
				}
			} else if isExported(d.Name.Name) {
				if d.Doc == nil || strings.TrimSpace(d.Doc.Text()) == "" {
					t.Errorf("%s: function %q has no godoc comment",
						short(path), d.Name.Name)
				}
			}
		case *ast.GenDecl:
			switch d.Tok {
			case token.TYPE:
				for _, spec := range d.Specs {
					ts := spec.(*ast.TypeSpec)
					if !isExported(ts.Name.Name) {
						continue
					}
					doc := ts.Doc
					if doc == nil {
						doc = d.Doc
					}
					if doc == nil || strings.TrimSpace(doc.Text()) == "" {
						t.Errorf("%s: type %q has no godoc comment",
							short(path), ts.Name.Name)
					}
				}
			case token.VAR, token.CONST:
				// For a grouped declaration (one `var (...)` block), a
				// comment on the group header satisfies every name
				// inside. Individual specs may also carry their own
				// comment. We require ONE or the other for at least one
				// exported name in the spec.
				for _, spec := range d.Specs {
					vs := spec.(*ast.ValueSpec)
					anyExported := false
					for _, n := range vs.Names {
						if isExported(n.Name) {
							anyExported = true
							break
						}
					}
					if !anyExported {
						continue
					}
					doc := vs.Doc
					if doc == nil {
						doc = d.Doc
					}
					if doc == nil || strings.TrimSpace(doc.Text()) == "" {
						var names []string
						for _, n := range vs.Names {
							if isExported(n.Name) {
								names = append(names, n.Name)
							}
						}
						t.Errorf("%s: identifier(s) %s have no godoc comment",
							short(path), strings.Join(names, ", "))
					}
				}
			}
		}
	}
}

func receiverIsExported(fl *ast.FieldList) bool {
	if fl == nil || len(fl.List) == 0 {
		return false
	}
	expr := fl.List[0].Type
	// Peel off pointer receiver.
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if ident, ok := expr.(*ast.Ident); ok {
		return isExported(ident.Name)
	}
	return false
}

func isExported(name string) bool {
	if name == "" {
		return false
	}
	r := []rune(name)[0]
	return unicode.IsUpper(r)
}

func short(p string) string {
	// Return "<pkg>/<file>.go" for readable output.
	if idx := strings.LastIndex(p, string(filepath.Separator)); idx >= 0 {
		parent := p[:idx]
		if idx2 := strings.LastIndex(parent, string(filepath.Separator)); idx2 >= 0 {
			return p[idx2+1:]
		}
	}
	return p
}
