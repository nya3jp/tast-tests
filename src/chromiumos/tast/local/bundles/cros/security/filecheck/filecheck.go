// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package filecheck helps tests check permissions and ownership of on-disk files.
package filecheck

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"chromiumos/tast/testing"
)

// Pattern matches one or more paths.
// It can be used to verify that matched paths have expected ownership and permissions.
type Pattern struct {
	match        Matcher
	uids, gids   []uint32     // allowed IDs; nil or empty to not check
	mode         *os.FileMode // mode perm bits must exactly match
	notMode      *os.FileMode // none of these perm bits may be set
	skipChildren bool         // should children (if this is a dir) be skipped?
}

// NewPattern returns a new Pattern that verifies that paths matched by m meet the requirements specified by rs.
func NewPattern(m Matcher, opts ...Option) *Pattern {
	pat := &Pattern{match: m}
	for _, o := range opts {
		o(pat)
	}
	return pat
}

// modeMask contains permission-related os.FileMode bits.
const modeMask = os.ModePerm | os.ModeSetuid | os.ModeSetgid | os.ModeSticky

// check inspects fi and returns a list of problems.
func (p *Pattern) check(fi os.FileInfo) (problems []string) {
	contains := func(allowed []uint32, id uint32) bool {
		for _, aid := range allowed {
			if id == aid {
				return true
			}
		}
		return false
	}

	st := fi.Sys().(*syscall.Stat_t)
	if len(p.uids) > 0 {
		if !contains(p.uids, st.Uid) {
			problems = append(problems, fmt.Sprintf("UID %v (want %v)", st.Uid, p.uids))
		}
	}
	if len(p.gids) > 0 {
		if !contains(p.gids, st.Gid) {
			problems = append(problems, fmt.Sprintf("GID %v (want %v)", st.Gid, p.gids))
		}
	}

	// Skip checking meaningless permissions on symbolic links.
	if fi.Mode()&os.ModeSymlink == 0 {
		mode := fi.Mode() & modeMask
		if p.mode != nil && mode != *p.mode {
			problems = append(problems, fmt.Sprintf("mode %04o (want %04o)", mode, *p.mode))
		}
		if p.notMode != nil {
			if bad := mode & *p.notMode; bad != 0 {
				problems = append(problems, fmt.Sprintf("mode %04o (%04o disallowed)", mode, bad))
			}
		}
	}

	return problems
}

func (p *Pattern) String() string {
	var fields []string
	if len(p.uids) > 0 {
		fields = append(fields, fmt.Sprintf("uids=%v", p.uids))
	}
	if len(p.gids) > 0 {
		fields = append(fields, fmt.Sprintf("gids=%d", p.gids))
	}
	if p.mode != nil {
		fields = append(fields, fmt.Sprintf("mode=%04o", *p.mode))
	}
	if p.notMode != nil {
		fields = append(fields, fmt.Sprintf("notMode=%04o", *p.notMode))
	}
	if p.skipChildren {
		fields = append(fields, "skipChildren")
	}
	return "[" + strings.Join(fields, " ") + "]"
}

// Option is used to configure a Pattern.
type Option func(*Pattern)

// UID requires that the path be owned by one of the supplied user IDs.
func UID(uids ...uint32) Option { return func(p *Pattern) { p.uids = uids } }

// GID requires that the path be owned by one of the supplied group IDs.
func GID(gids ...uint32) Option { return func(p *Pattern) { p.gids = gids } }

// checkMode panics if m contains any non-permission-related bits.
func checkMode(m os.FileMode) {
	if invalid := m & ^modeMask; invalid != 0 {
		panic(fmt.Sprintf("invalid bit(s) %04o", m))
	}
}

// Mode requires that permission-related bits in the path's mode exactly match m.
// Only 0777, setuid, setgid, and the sticky bit may be supplied.
func Mode(m os.FileMode) Option {
	return func(p *Pattern) {
		checkMode(m)
		p.mode = &m
	}
}

// NotMode requires that the permission-related bits in the path's mode contain none of the bits in nm.
// Only 0777, setuid, setgid, and the sticky bit may be supplied.
func NotMode(nm os.FileMode) Option {
	return func(p *Pattern) {
		checkMode(nm)
		p.notMode = &nm
	}
}

// SkipChildren indicates that any child paths should not be checked.
// The directory itself will still be checked. This has no effect for non-directories.
func SkipChildren() Option { return func(p *Pattern) { p.skipChildren = true } }

// Matcher matches a path relative to the root passed to Check.
type Matcher func(path string) bool

// AllPaths returns a Matcher that matches all paths.
func AllPaths() Matcher {
	return func(p string) bool { return true }
}

// Path returns a Matcher that matches only the supplied path (relative to the root passed to Check).
func Path(path string) Matcher {
	if path == "" || path[0] == '/' {
		panic("Path must be relative")
	}
	return func(p string) bool { return p == path }
}

// Root returns a Matcher that matches the root path passed to Check.
func Root() Matcher {
	return func(p string) bool { return p == "" }
}

// PathRegexp returns a Matcher that matches all paths matched by regular expression r.
// r is evaluated against paths relative to the root passed to Check.
func PathRegexp(r string) Matcher {
	re := regexp.MustCompile(r)
	return func(p string) bool { return re.MatchString(p) }
}

// Tree returns a Matcher that matches both path and its children.
// The path is relative to the root passed to Check.
func Tree(path string) Matcher {
	if path == "" {
		panic("Use AllPaths to match all paths")
	}
	pre := path + "/"
	return func(p string) bool { return p == path || strings.HasPrefix(p, pre) }
}

// Check inspects all files within (and including) root.
// pats are executed in-order against each path.
// If a pattern matches a path, no later patterns are evaluated against it.
// If SkipChildren is included in a pattern , any matched directories' children are skipped.
// A map from absolute path names to strings describing problems is returned,
// along with the number of paths (not including ones skipped by SkipChildren) that were inspected.
func Check(ctx context.Context, root string, pats []*Pattern) (
	problems map[string][]string, numPaths int, err error) {
	problems = make(map[string][]string)
	err = filepath.Walk(root, func(fullPath string, fi os.FileInfo, err error) error {
		// Check for test timeout.
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// If filepath.Walk encountered an error inspecting the file, skip it.
		// This generally seems to happen due to a file getting deleted mid-run, but we also sometimes
		// see "readdirent: input/output error" errors: https://crbug.com/908416
		if err != nil {
			testing.ContextLogf(ctx, "Failed to check %v: %v", fullPath, err)
			return nil
		}

		relPath := ""
		if fullPath != root {
			relPath = fullPath[len(root+"/"):]
		}
		numPaths++

		for _, pat := range pats {
			if pat.match(relPath) {
				if msgs := pat.check(fi); len(msgs) > 0 {
					problems[fullPath] = append(problems[fullPath], msgs...)
				}
				if pat.skipChildren && fi.IsDir() {
					return filepath.SkipDir
				}
				break
			}
		}

		return nil
	})

	return problems, numPaths, err
}
