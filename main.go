package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
)

type Tag struct {
	Name  string
	File  string
	Start string
	End   string
	Type  string
}

// Tag types.
const (
	Method   string = "method"
	Function string = "function"
)

type tagParser struct {
	fset  *token.FileSet
	tags  []*Tag
	types []string // all types we encounter, used to determine the constructors
}

func Parse(filename string) ([]*Tag, error) {
	p := &tagParser{
		fset:  token.NewFileSet(),
		tags:  []*Tag{},
		types: make([]string, 0),
	}

	f, err := parser.ParseFile(p.fset, filename, nil, 0)
	if err != nil {
		return nil, err
	}
	p.parseDeclarations(f)

	return p.tags, nil
}

func (p *tagParser) parseDeclarations(f *ast.File) {
	for _, d := range f.Decls {
		if decl, ok := d.(*ast.FuncDecl); ok {
			p.parseFunc(decl)
		}
	}
}

func (p *tagParser) parseFunc(f *ast.FuncDecl) {
	tag := p.createTag(f.Name.Name, f.Pos(), f.End(), Function)

	if f.Recv != nil && len(f.Recv.List) > 0 {
		// this function has a receiver, set the type to Method
		tag.Type = Method
	} else if _, ok := p.belongsToReceiver(f.Type.Results); ok {
		// this function does not have a receiver, but it belongs to one based
		// on its return values; its type will be Function instead of Method.
		tag.Type = Function
	}
	p.tags = append(p.tags, tag)
}

func (p *tagParser) createTag(name string, start, end token.Pos, tagType string) *Tag {
	f := p.fset.File(start).Name()
	return &Tag{
		Name:  name,
		File:  f,
		Start: strconv.Itoa(p.fset.Position(start).Line),
		End:   strconv.Itoa(p.fset.Position(end).Line),
		Type:  tagType,
	}
}

// belongsToReceiver checks if a function with these return types belongs to
// a receiver. If it belongs to a receiver, the name of that receiver will be
// returned with ok set to true. Otherwise ok will be false.
// Behavior should be similar to how go doc decides when a function belongs to
// a receiver (gosrc/pkg/go/doc/reader.go).
func (p *tagParser) belongsToReceiver(types *ast.FieldList) (name string, ok bool) {
	if types == nil || types.NumFields() == 0 {
		return "", false
	}

	// If the first return type has more than 1 result associated with
	// it, it should not belong to that receiver.
	// Similar behavior as go doc (go source/.
	if len(types.List[0].Names) > 1 {
		return "", false
	}

	// get name of the first return type
	t := getType(types.List[0].Type, false)

	// check if it exists in the current list of known types
	for _, knownType := range p.types {
		if t == knownType {
			return knownType, true
		}
	}

	return "", false
}

func getType(node ast.Node, star bool) (paramType string) {
	switch t := node.(type) {
	case *ast.Ident:
		paramType = t.Name
	case *ast.StarExpr:
		if star {
			paramType = "*" + getType(t.X, star)
		} else {
			paramType = getType(t.X, star)
		}
	case *ast.SelectorExpr:
		paramType = getType(t.X, star) + "." + getType(t.Sel, star)
	case *ast.ArrayType:
		if l, ok := t.Len.(*ast.BasicLit); ok {
			paramType = fmt.Sprintf("[%s]%s", l.Value, getType(t.Elt, star))
		} else {
			paramType = "[]" + getType(t.Elt, star)
		}
	case *ast.FuncType:
		fparams := getTypes(t.Params, true)
		fresult := getTypes(t.Results, false)

		if len(fresult) > 0 {
			paramType = fmt.Sprintf("func(%s) %s", fparams, fresult)
		} else {
			paramType = fmt.Sprintf("func(%s)", fparams)
		}
	case *ast.MapType:
		paramType = fmt.Sprintf("map[%s]%s", getType(t.Key, true), getType(t.Value, true))
	case *ast.ChanType:
		paramType = fmt.Sprintf("chan %s", getType(t.Value, true))
	case *ast.InterfaceType:
		paramType = "interface{}"
	}
	return
}

// getTypes returns a comma separated list of types in fields. If includeNames is
// true each type is preceded by a comma separated list of parameter names.
func getTypes(fields *ast.FieldList, includeNames bool) string {
	if fields == nil {
		return ""
	}

	types := make([]string, len(fields.List))
	for i, param := range fields.List {
		if len(param.Names) > 0 {
			// there are named parameters, there may be multiple names for a single type
			t := getType(param.Type, true)

			if includeNames {
				// join all the names, followed by their type
				names := make([]string, len(param.Names))
				for j, n := range param.Names {
					names[j] = n.Name
				}
				t = fmt.Sprintf("%s %s", strings.Join(names, ", "), t)
			} else {
				if len(param.Names) > 1 {
					// repeat t len(param.Names) times
					t = strings.Repeat(fmt.Sprintf("%s, ", t), len(param.Names))

					// remove trailing comma and space
					t = t[:len(t)-2]
				}
			}

			types[i] = t
		} else {
			// no named parameters
			types[i] = getType(param.Type, true)
		}
	}

	return strings.Join(types, ", ")
}

var flags = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

func main() {
	if err := flags.Parse(os.Args[1:]); err == flag.ErrHelp {
		return
	}
	tags := []*Tag{}
	for _, file := range flags.Args() {
		ts, err := Parse(file)
		if err != nil {
			continue
		}
		tags = append(tags, ts...)
	}
	for _, v := range tags {
		fmt.Println(v)
	}
}
