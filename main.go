package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"
)

var order = map[token.Token]int{
	token.IMPORT: 0,
	token.CONST:  1,
	token.VAR:    2,
	token.TYPE:   3,
	token.FUNC:   4,
}

type Config struct {
	SortAlphabetically bool
}

type funcOrMethod struct {
	name string
	recv string
}

func assignCommentsToDecl(tree *ast.File, content []byte) map[ast.Decl][]byte {
	comments := map[ast.Decl][]byte{
		nil: {'\n'},
	}

	for _, c := range tree.Comments {
		start, end := c.Pos(), c.End()

		// skip doc comments
		if start < tree.Package {
			continue
		}

		// skip comments within declarations
		isRootComment := true
		for _, d := range tree.Decls {
			if d.Pos() <= start && end <= d.End() {
				isRootComment = false
				break
			}
		}

		if !isRootComment {
			continue
		}

		var found bool
		for _, d := range tree.Decls {
			if d.Pos() > c.End() {
				comment := content[start-1 : end]
				for i := int(end); i < len(content); i++ {
					if content[i] == '\n' {
						comment = append(comment, '\n')
					} else {
						break
					}
				}
				comments[d] = append(comments[d], comment...)
				found = true
				break
			}
		}

		if !found {
			comments[nil] = append(comments[nil], content[start-1:]...)
		}
	}

	return comments
}

// funcName returns the function name in the form of "<receiver type> <function name>"
// e.g. funcName("func (f Foo) String() {}") = {recv: "Foo", name: "String"}
// a function without a receiver
func funcName(f *ast.FuncDecl) funcOrMethod {
	name := f.Name.Name
	if f.Recv == nil || len(f.Recv.List) == 0 {
		return funcOrMethod{name: name}
	}

	var recv string
	recvType := f.Recv.List[0].Type
	switch recvType := recvType.(type) {
	case *ast.StarExpr:
		recv = recvType.X.(*ast.Ident).Name
	case *ast.Ident:
		recv = recvType.Name
	default:
		panic("invalid receiver type: " + reflect.TypeOf(recvType).String())
	}

	return funcOrMethod{recv: recv, name: name}
}

func getToken(d ast.Decl) token.Token {
	switch d := d.(type) {
	case *ast.FuncDecl:
		return token.FUNC
	case *ast.GenDecl:
		return d.Tok
	default:
		fmt.Printf("bad declaration: %v\n", reflect.TypeOf(d))
		panic("unimpl for")
	}
}

func run() error {
	var (
		config Config
		help   bool
	)

	flag.BoolVar(&help, "h", false, "help")
	flag.BoolVar(&config.SortAlphabetically, "a", false, "sort alphabetically")
	flag.Parse()

	if help {
		flag.Usage()
		return nil
	}

	contents, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("failed to read from stdin: %w", err)
	}

	buf, err := sortFile(contents, config)

	fmt.Println(buf.String())

	return nil
}

func sortAST(t *ast.File, conf Config) error {
	sort.Slice(t.Decls, func(i, j int) bool {
		a, b := t.Decls[i], t.Decls[j]
		// sort types first
		aType, bType := getToken(a), getToken(b)
		if aType != bType {
			return order[aType] < order[bType]
		}

		if conf.SortAlphabetically {
			// two consecutive functions are sorted alphabetically by their name
			if a, ok := a.(*ast.FuncDecl); ok {
				if b, ok := b.(*ast.FuncDecl); ok {
					a, b := funcName(a), funcName(b)
					// main function goes last
					if a.recv == "" && a.name == "main" {
						return false
					} else if b.recv == "" && b.name == "main" {
						return true
					}

					// functions go after methods
					if a.recv == "" && b.recv != "" {
						return false
					}
					if b.recv == "" && a.recv != "" {
						return true
					}

					// sort methods based on the receiver
					if a.recv != b.recv {
						return strings.Compare(a.recv, b.recv) < 0
					}

					// sort functions and methods alphabetically
					return strings.Compare(a.name, b.name) < 0
				}
			}
			// two consecutive general declarations
			if a, ok := a.(*ast.GenDecl); ok {
				if b, ok := b.(*ast.GenDecl); ok {
					// two individual declarations!
					if len(a.Specs) == 1 && len(b.Specs) == 1 {
						var getName func(s ast.Spec) string
						// type decl
						if a.Tok == token.TYPE && b.Tok == token.TYPE {
							getName = func(s ast.Spec) string {
								return s.(*ast.TypeSpec).Name.Name
							}
						} else if a.Tok == token.VAR && b.Tok == token.VAR || a.Tok == token.CONST && b.Tok == token.CONST {
							getName = func(s ast.Spec) string {
								return s.(*ast.ValueSpec).Names[0].Name
							}
						}

						if getName != nil {
							a, b := getName(a.Specs[0]), getName(b.Specs[0])
							return strings.Compare(a, b) < 0
						}
					}
				}
			}
		}

		// keep in the same order
		return false
	})
	return nil
}

// last comments
func sortFile(contents []byte, config Config) (*bytes.Buffer, error) {
	ast, err := parser.ParseFile(
		token.NewFileSet(),
		"", contents,
		parser.ParseComments|parser.AllErrors,
	)

	if err != nil {
		return nil, fmt.Errorf("failed paring file to AST: %w", err)
	}

	comments := assignCommentsToDecl(ast, contents)

	err = sortAST(ast, config)
	if err != nil {
		return nil, fmt.Errorf("failed to sort AST: %w", err)
	}

	buf := toFileBytes(ast, contents, comments)

	return buf, nil
}

// skip doc comments
func toFileBytes(tree *ast.File, contents []byte, comments map[ast.Decl][]byte) *bytes.Buffer {
	w := &bytes.Buffer{}

	if tree.Doc != nil {
		for _, each := range tree.Doc.List {
			w.WriteString(each.Text + "\n")
		}
	}

	fmt.Fprintf(w, "package %s\n\n", tree.Name)

	for i, decl := range tree.Decls {
		// trailing comments
		if comments, ok := comments[decl]; ok {
			w.Write(comments)
		}

		// declaration itself
		w.Write(contents[decl.Pos()-1 : decl.End()-1])

		// leading new lines
		if i < len(tree.Decls)-1 {
			w.WriteString("\n\n")
		}
	}

	if comments, ok := comments[nil]; ok {
		w.Write(comments)
	}
	return w
}

func main() {
	if err := run(); err != nil {
		fmt.Println(err)
	}
}
