package main

import (
	"go/build"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

type DepType uint8

const (
	DepTypeMain DepType = 1 << iota
	DepTypeTest
)

type Link struct {
	DepType DepType
	Node    *Node
}

type Node struct {
	Name    string
	Dir     string
	Imports map[string]Link
}

type Graph struct {
	Entries map[string]*Node
}

func BuildDepGraph(pkgs []string, basedir string) (*Graph, error) {
	n := &nursery{
		cwd:      basedir,
		ctx:      build.Default,
		seen:     make(map[[2]string]*Node),
		byPath:   make(map[string]*Node),
		basedirs: make(map[string]string),
	}

	var err error

	f := &Graph{Entries: make(map[string]*Node)}
	for _, path := range pkgs {
		f.Entries[path], err = n.visit(basedir, path)
		if err != nil {
			return nil, errors.Wrap(err, "could not build dependency tree for "+path)
		}
	}

	return f, nil
}

type nursery struct {
	cwd      string
	ctx      build.Context
	seen     map[[2]string]*Node
	byPath   map[string]*Node
	basedirs map[string]string
}

// TODO: Implement proper CGO support. For now, users will have to manually
// specify their CGO deps.
var cgo = &Node{Name: "C", Imports: make(map[string]Link)}

// visit visits a node and its children in depth-first order, so we know that as
// we process the node itself, all of its deps are already seen.
func (n *nursery) visit(basedir, path string) (*Node, error) {
	if path == "C" {
		return cgo, nil
	}

	var bds []string
	td := basedir
Loop:
	for {
		if bd, ok := n.basedirs[td]; ok {
			td = bd
			break
		}
		hv, err := hasVendor(td)
		switch {
		case err != nil:
			return nil, errors.Wrap(err, "could not check vendor dir")
		case !hv:
			next := filepath.Dir(td)
			if next == n.ctx.GOPATH || next == filepath.Join(n.ctx.GOROOT, "src") {
				break Loop
			}

			bds = append(bds, td)
			td = next
		default:
			break Loop
		}
	}

	for _, bd := range bds {
		n.basedirs[bd] = td
	}

	basedir = td

	if node, ok := n.seen[[2]string{basedir, path}]; ok {
		return node, nil
	}

	pkg, err := n.ctx.Import(path, basedir, 0)
	if err != nil {
		return nil, errors.Wrap(err, "could not visit node "+path)
	}

	if cNode, ok := n.byPath[pkg.Dir]; ok {
		n.seen[[2]string{basedir, path}] = cNode
		return cNode, nil

	}

	node := &Node{
		Name:    path,
		Dir:     pkg.Dir,
		Imports: make(map[string]Link),
	}
	n.seen[[2]string{basedir, path}] = node
	n.byPath[pkg.Dir] = node

	for _, i := range pkg.Imports {
		child, err := n.visit(pkg.Dir, i)
		if err != nil {
			return nil, errors.Wrap(err, "could not visit child node of "+path)
		}

		node.Imports[i] = Link{
			DepType: DepTypeMain,
			Node:    child,
		}
	}

	for _, i := range pkg.TestImports {
		child, err := n.visit(pkg.Dir, i)
		if err != nil {
			return nil, errors.Wrap(err, "could not visit child node of "+path)
		}

		link, ok := node.Imports[i]

		if !ok {
			link = Link{
				Node: child,
			}
		}

		link.DepType |= DepTypeTest
		node.Imports[i] = link
	}

	return node, nil
}

func hasVendor(path string) (bool, error) {
	_, err := os.Stat(filepath.Join(path, "vendor"))
	switch {
	case err == nil:
		return true, nil
	case os.IsNotExist(err):
		return false, nil
	default:
		return false, err
	}
}

// TopoWalk visits every node in each tree in the graph in reverse topological order.
func (f *Graph) TopoWalk(visit func(*Node)) {
	visited := make(map[*Node]struct{})

	var topovisit func(n *Node)
	topovisit = func(n *Node) {
		if _, ok := visited[n]; ok {
			return
		}

		visited[n] = struct{}{}
		for _, c := range n.Imports {
			topovisit(c.Node)
		}

		visit(n)
	}

	for _, n := range f.Entries {
		topovisit(n)
	}
}
