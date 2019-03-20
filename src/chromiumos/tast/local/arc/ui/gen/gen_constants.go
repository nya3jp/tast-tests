// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package main implements a script for writing a Go source file containing input event constants.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

const (
	// Passed to writeConstants() to feed the template.
	goGen = `// Assumes that Android repo is checked out at same folder level as Chrome OS. e.g: If Chrome OS sources are in:
// ~/src/chromeos/, then Android sources should be in ~/src/android/
//go:generate go run gen/gen_constants.go gen/util.go ../../../../../../../../../../android/frameworks/base/core/java/android/view/KeyEvent.java generated_constants.go
//go:generate go fmt generated_constants.go`
	pkgName  = "ui"
	repoName = "Android frameworks/base repo"
)

const (
	// Type names used in typeInfo.
	keyCodeType = "KeyCodeType"
	metaState   = "MetaState"
)

// getGroupForName returns group info for the supplied constant.
// groups represents the map the contains the different groups.
func getGroupForName(groups []*groupInfo, name string) *groupInfo {
	for _, g := range groups {
		if strings.HasPrefix(name, g.prefix) {
			return g
		}
	}
	return nil
}

// readConstants reads path, a KeyEvent.java file, and returns a subset of relevant constants from it.
// groups represents the map the contains the different groups.
func readConstants(groups []*groupInfo, path string) (constantGroups, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	consts := make(constantGroups)

	// Looking for:
	//  public static final int META_SELECTING = 0x800;
	// TODO(ricardoq): Multiline, bitwise-or metas are not supported. Find a robust way to support them. e.g:
	//   public static final int META_SHIFT_MASK = META_SHIFT_ON
	//        | META_SHIFT_LEFT_ON | META_SHIFT_RIGHT_ON;
	re := regexp.MustCompile(`^\s+public static final int ([_A-Z0-9]+)\s*=\s*(0x[0-9a-fA-F]+|\d+);$`)

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		matches := re.FindStringSubmatch(sc.Text())
		if matches == nil {
			continue
		}
		name, sval := matches[1], matches[2]
		grp := getGroupForName(groups, name)
		if grp == nil {
			// It is safe to silently ignore unsupported groups.
			continue
		}

		base := 10
		if len(sval) > 2 && sval[:2] == "0x" {
			base = 16
			sval = sval[2:] // strconv.ParseInt doesn't want "0x" prefix
		}
		var val int64
		if val, err = strconv.ParseInt(sval, base, 64); err != nil {
			return nil, errors.Wrapf(err, "unable to parse value %q for %q", sval, name)
		}
		consts[grp.prefix] = append(consts[grp.prefix], constant{name, val})
	}

	// Sort each group by ascending value.
	for _, cs := range consts {
		sort.Slice(cs, func(i, j int) bool { return cs[i].val < cs[j].val })
	}

	return consts, sc.Err()
}

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <KeyEvent.java> <out.go>\n", os.Args[0])
		os.Exit(1)
	}

	inputFile := args[0]
	outputFile := args[1]

	repoPath, repoRev, err := getRepoInfo(inputFile)
	if err != nil {
		log.Fatalf("Failed to get repo info for %v: %v", inputFile, err)
	}

	var types = []*typeInfo{
		&typeInfo{keyCodeType, "uint16", "represents an Android key code."},
		&typeInfo{metaState, "uint64", "represents a meta-key state. Each bit set to 1 represents a pressed meta key."},
	}

	// We only care about KEYCODE and META prefixes. We ignore the rest.
	var groups = []*groupInfo{
		&groupInfo{"KEYCODE", keyCodeType, "KeyCodes constants"},
		&groupInfo{"META", metaState, "Meta-key constants"},
	}

	kv := tmplArgs{
		repoPathKey:    repoPath,
		repoRevKey:     repoRev,
		repoNameKey:    repoName,
		packageNameKey: pkgName,
		preludeCodeKey: goGen,
	}

	consts, err := readConstants(groups, inputFile)
	if err != nil {
		log.Fatalf("Failed to read %v: %v", inputFile, err)
	}

	if err := writeConstants(consts, groups, types, kv, outputFile); err != nil {
		log.Fatalf("Failed to write %v: %v", outputFile, err)
	}
}
