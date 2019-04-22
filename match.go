package main

import (
	"path/filepath"
	"strings"
)

type pkgMatcher struct {
	path       string
	isRelative bool
	isPrefix   bool
}

func newPkgMatcher(pattern string) pkgMatcher {
	p := pkgMatcher{}

	if pattern[:2] == "./" {
		p.isRelative = true
		pattern = pattern[2:]
	}

	if pattern[len(pattern)-3:] == "..." {
		p.isPrefix = true
		pattern = pattern[:len(pattern)-3]
	}

	return p
}

func (p pkgMatcher) Match(cwd, path string) bool {
	if strings.Contains(path, "/vendor/") {
		return false
	}

	pattern := p.path
	if p.isRelative {
		pattern = filepath.Join(cwd, pattern)
	}

	if p.isPrefix {
		return strings.HasPrefix(path, pattern)
	}

	return path == pattern
}
