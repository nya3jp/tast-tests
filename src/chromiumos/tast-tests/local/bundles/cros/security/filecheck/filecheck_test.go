// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filecheck

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
	"time"

	"chromiumos/tast/testutil"
)

// fakeFileInfo is an implementation of os.FileInfo used for unit tests.
type fakeFileInfo struct {
	mode os.FileMode
	st   syscall.Stat_t
}

func (fi *fakeFileInfo) Name() string       { return "" }
func (fi *fakeFileInfo) Size() int64        { return 0 }
func (fi *fakeFileInfo) Mode() os.FileMode  { return fi.mode }
func (fi *fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (fi *fakeFileInfo) IsDir() bool        { return false }
func (fi *fakeFileInfo) Sys() interface{}   { return &fi.st }

func TestOptions(t *testing.T) {
	const (
		mode = os.ModeSticky | 0755
		uid  = 1000
		gid  = 2000
	)
	fi := &fakeFileInfo{mode, syscall.Stat_t{Uid: uid, Gid: gid}}

	for _, tc := range []struct {
		opts     []Option
		numProbs int
	}{
		{[]Option{}, 0},
		{[]Option{UID(uid)}, 0},
		{[]Option{GID(gid)}, 0},
		{[]Option{UID(uid), GID(gid)}, 0},
		{[]Option{UID(uid + 1), GID(gid)}, 1},
		{[]Option{UID(uid + 1), GID(gid + 1)}, 2},
		{[]Option{UID(uid, uid+1)}, 0},
		{[]Option{UID(uid+1, uid+2)}, 1},
		{[]Option{GID(gid, gid+1)}, 0},
		{[]Option{GID(gid+1, gid+2)}, 1},
		{[]Option{Mode(mode)}, 0},
		{[]Option{Mode(0755)}, 1},
		{[]Option{NotMode(os.ModeSetuid)}, 0},
		{[]Option{NotMode(os.ModeSticky)}, 1},
	} {
		pat := NewPattern(AllPaths(), tc.opts...)
		probs := pat.check(fi)
		if len(probs) != tc.numProbs {
			t.Errorf("%v returned %v; want %v problem(s)", pat.String(), probs, tc.numProbs)
		}
	}
}

func TestMatchers(t *testing.T) {
	for i, tc := range []struct {
		matcher Matcher
		path    string
		matched bool
	}{
		{Path("a/b"), "a/b", true},
		{Path("a/b/c"), "a/b", false},
		{Path("a/b"), "a/b/c", false},
		{Root(), "", true},
		{Root(), "a", false},
		{Root(), "a/b", false},
		{PathRegexp("^a/b"), "a/b", true},
		{PathRegexp("^a/b"), "a/bc", true},
		{PathRegexp("^a/b"), "a/blah", true},
		{PathRegexp("^a/b"), "a", false},
		{Tree("a/b"), "a", false},
		{Tree("a/b"), "a/b", true},
		{Tree("a/b"), "a/b/c", true},
		{Tree("a/b"), "ab/c", false},
		{Tree("a/b"), "a/bc", false},
	} {
		if matched := tc.matcher(tc.path); matched != tc.matched {
			t.Errorf("%d: match of %q = %v; want %v", i, tc.path, matched, tc.matched)
		}
	}
}

func TestCheck(t *testing.T) {
	td := testutil.TempDir(t)
	defer os.RemoveAll(td)

	if err := testutil.WriteFiles(td, map[string]string{
		"checked":           "", // not skipped
		"skip-dir/file":     "", // skipped for dir
		"skip-tree/file":    "", // skipped for tree
		"skip-file":         "", // skipped for full path
		"skip-pre-file":     "", // skipped for regexp
		"skip-pre-dir/file": "", // skipped for regexp
	}); err != nil {
		t.Fatal(err)
	}

	// Returns absolute path to relative path rel within td.
	abs := func(rel string) string { return filepath.Join(td, rel) }

	// Find the UID/GID of the files we created and choose arbitrary other IDs.
	fi, err := os.Stat(abs("checked"))
	if err != nil {
		t.Fatal(err)
	}
	st := fi.Sys().(*syscall.Stat_t)
	ourUID := st.Uid
	uidPass := UID(ourUID)
	uidFail := UID(ourUID + 1)
	ourGID := st.Gid
	gidPass := GID(ourGID)
	gidFail := GID(ourGID + 1)

	// Declare patterns to make us avoid checking all dirs and files except the "checked" file.
	basePatterns := []*Pattern{
		NewPattern(Root()),
		NewPattern(Path("skip-dir"), SkipChildren()),
		NewPattern(Path("skip-file")),
		NewPattern(Tree("skip-tree")),
		NewPattern(PathRegexp("^skip-pre-")),
	}

	for _, tc := range []struct {
		pats     []*Pattern
		numProbs map[string]int // path -> number of expected problems
	}{
		{[]*Pattern{}, map[string]int{}},                                                          // no requirements
		{[]*Pattern{NewPattern(AllPaths(), uidPass, gidPass)}, map[string]int{}},                  // requirements are met
		{[]*Pattern{NewPattern(AllPaths(), uidFail)}, map[string]int{abs("checked"): 1}},          // bad UID
		{[]*Pattern{NewPattern(AllPaths(), gidFail)}, map[string]int{abs("checked"): 1}},          // bad GID
		{[]*Pattern{NewPattern(AllPaths(), uidFail, gidFail)}, map[string]int{abs("checked"): 2}}, // bad UID and GID
	} {
		pats := append([]*Pattern{}, basePatterns...)
		pats = append(pats, tc.pats...)
		probs, _, err := Check(context.Background(), td, pats)
		if err != nil {
			t.Errorf("Check(ctx, %q, %+v) failed: %v", td, pats, err)
			continue
		}
		numProbs := make(map[string]int, len(probs))
		for p, msgs := range probs {
			numProbs[p] = len(msgs)
		}
		if !reflect.DeepEqual(numProbs, tc.numProbs) {
			t.Errorf("Check(ctx, %q, %+v) = %v; want counts %v", td, pats, probs, tc.numProbs)
		}
	}
}
