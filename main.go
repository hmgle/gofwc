package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
)

type Tag struct {
	Name     string
	File     string
	Start    string
	End      string
	Type     string
	RecvType []string
	RecvName [][]string
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
		for i, v := range f.Recv.List {
			tag.RecvType = append(tag.RecvType, fmt.Sprint(v.Type))
			for _, v2 := range v.Names {
				tag.RecvName[i] = append(tag.RecvName[i], v2.Name)
			}
		}
	}
	p.tags = append(p.tags, tag)
}

func (p *tagParser) createTag(name string, start, end token.Pos, tagType string) *Tag {
	f := p.fset.File(start).Name()
	return &Tag{
		Name:     name,
		File:     f,
		Start:    strconv.Itoa(p.fset.Position(start).Line),
		End:      strconv.Itoa(p.fset.Position(end).Line),
		Type:     tagType,
		RecvType: []string{},
		RecvName: [][]string{{}},
	}
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
