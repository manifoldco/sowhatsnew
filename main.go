package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jessevdk/go-flags"
)

var opts struct {
	From string `short:"f" long:"from" description:"Search from this commit. If not provided, the value is detected from the build environment"`
	To   string `short:"t" long:"to" description:"Search to this commit. If not provided, the value is detected from the build environment"`

	Config string `short:"c" long:"config" description:"Optional config file location" default:".sowhatsnew.hcl"`
}

func main() {
	parser := flags.NewParser(&opts, flags.Default)
	parser.Name = "sowhatsnew"
	parser.ShortDescription = "Detect code changes in Go packages for delta builds"

	if _, err := parser.Parse(); err != nil {
		fmt.Fprintln(os.Stderr, "could not read arguments:", err)
		os.Exit(1)
	}

	rawCFG, err := ioutil.ReadFile(opts.Config)
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "could not read config file:", err)
		os.Exit(1)
	}

	cfg, err := NewConfig(string(rawCFG))
	if err != nil {
		fmt.Fprintln(os.Stderr, "error parsing config:", err)
		os.Exit(1)
	}

	var pkgs []string

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		pkgs = append(pkgs, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "could not read package list:", err)
		os.Exit(1)
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "could not determine working directory:", err)
		os.Exit(1)
	}

	var repo GitRepository

	isGit, err := repo.Detect()
	switch {
	case !isGit:
		fmt.Fprintln(os.Stderr, "not a git repository root")
		os.Exit(2)
	case err != nil:
		fmt.Fprintln(os.Stderr, "could not detect repository type:", err)
		os.Exit(1)
	}

	from, to := commitRange(opts.From, opts.To)
	if from == "" && to == "" {
		outputResults(pkgs)
		return
	}

	mf, err := repo.ModifiedFiles(from, to)
	if err != nil {
		fmt.Fprintln(os.Stderr, "could not determine modified files:", err)
		os.Exit(1)
	}

	modifiedGoDirs := make(map[string]struct{})
	for _, f := range mf {
		if filepath.Ext(f) == ".go" { // XXX: consider tests
			modifiedGoDirs[filepath.Join(cwd, filepath.Dir(f))] = struct{}{}
		}
	}

	var explicitDeps []pkgMatcher
	for _, f := range mf {
		if v, ok := cfg.Deps[f]; ok {
			explicitDeps = append(explicitDeps, newPkgMatcher(v))
		}
	}

	g, err := BuildDepGraph(pkgs, cwd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "could not build dependency trees:", err)
		os.Exit(1)
	}

	needsRebuild := make(map[*Node]struct{})

	g.TopoWalk(func(n *Node) {
		for _, c := range n.Imports {
			if _, ok := needsRebuild[c.Node]; ok {
				needsRebuild[n] = struct{}{}
				return
			}
		}

		if _, ok := modifiedGoDirs[n.Dir]; ok {
			needsRebuild[n] = struct{}{}
		}

		for _, e := range explicitDeps {
			if e.Match(cwd, n.Dir) {
				needsRebuild[n] = struct{}{}
				break
			}
		}
	})

	nr := make([]string, 0, len(needsRebuild))
	for p := range needsRebuild {
		if strings.HasPrefix(p.Dir, cwd) {
			nr = append(nr, p.Name)
		}
	}

	outputResults(pkgs)
}

func outputResults(pkgs []string) {
	sort.Strings(pkgs)

	for _, p := range pkgs {
		fmt.Println(p)
	}
}
