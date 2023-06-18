package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"sort"
	"strings"
)

var orderMap = map[token.Token]int{
	token.IMPORT: 0,
	token.CONST:  1,
	token.VAR:    2,
	token.TYPE:   3,
	token.FUNC:   4,
}

type Config struct {
	SortAlphabetically bool
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
		config  Config
		inPlace bool
		help    bool
	)

	flag.BoolVar(&inPlace, "w", false, "write result to source file instead of stdout")
	flag.BoolVar(&help, "h", false, "help")
	flag.BoolVar(&config.SortAlphabetically, "a", false, "sort alphabetically")
	flag.Parse()

	if help {
		flag.Usage()
		return nil
	}

	if flag.NArg() != 1 {
		return errors.New("exactly one argument required")
	}

	filePath := flag.Arg(0)

	contents, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	buf, err := sortFile(contents, config)

	if inPlace {
		if err := os.WriteFile(filePath, buf.Bytes(), 0666); err != nil {
			return fmt.Errorf("failed to write to file: %w", err)
		}
	} else {
		fmt.Println(buf.String())
	}

	return nil
}

func sortAST(t *ast.File, conf Config) error {
	sort.Slice(t.Decls, func(i, j int) bool {
		a, b := t.Decls[i], t.Decls[j]
		// sort types first
		aType, bType := getToken(a), getToken(b)
		if aType != bType {
			return orderMap[aType] < orderMap[bType]
		}

		if conf.SortAlphabetically {
			if a, ok := a.(*ast.FuncDecl); ok {
				// two consecutive functions are sorted alphabetically by their name
				if b, ok := b.(*ast.FuncDecl); ok {
					aName, bName := a.Name.Name, b.Name.Name
					// main function goes last
					if aName == "main" {
						return false
					} else if bName == "main" {
						return true
					}
					return strings.Compare(aName, bName) < 0
				}
			}
		}

		// keep in the same order
		return true
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
