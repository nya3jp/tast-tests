// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package main implements a script for writing a Go source file containing input event constants.
package main

import (
	"fmt"
	"os"
)

const (
	// Which package do the generated constants belong to.
	pkgName = "ui"

	// Looking for:
	//  public static final int META_SELECTING = 0x800;
	// TODO(ricardoq): Multiline, bitwise-or metas are not supported. Find a robust way to support them. e.g:
	//   public static final int META_SHIFT_MASK = META_SHIFT_ON
	//        | META_SHIFT_LEFT_ON | META_SHIFT_RIGHT_ON;
	regexpStr = `^\s+public static final int ([_A-Z0-9]+)\s*=\s*(0x[0-9a-fA-F]+|\d+);$`

	// go:generate rule to autogenerate the constants.
	goGenStr = `// Assumes that Android repo is checked out at same folder level as Chrome OS. e.g: If Chrome OS sources are in:
// ~/src/chromeos/, then Android sources should be in ~/src/android/
//go:generate go run gen/gen_constants.go gen/util.go ../../../../../../../../../../android/frameworks/base/core/java/android/view/KeyEvent.java generated_constants.go
//go:generate go fmt generated_constants.go`

	// The Git repository from where the constants were taken from.
	repoName = "Android frameworks/base repo"

	keyCodeType = "KeyCodeType"
	metaState   = "MetaState"
)

var types []*typeInfo = []*typeInfo{
	&typeInfo{keyCodeType, "uin16", "represents an Android key code."},
	&typeInfo{metaState, "uint64", "represents a meta-key state. Each bit set to 1 represents a pressed meta key."},
}

var groups []*groupInfo = []*groupInfo{
	&groupInfo{"KEYCODE", keyCodeType, "KeyCodes constants"},
	&groupInfo{"META", metaState, "Meta-key constants"},
}

func main() {
	if len(os.Args) != 3 || os.Args[1] == "" || os.Args[1][0] == '-' || os.Args[2] == "" || os.Args[2][0] == '-' {
		fmt.Fprintf(os.Stderr, "Usage: %s <KeyEvent.java> <out.go>\n", os.Args[0])
		os.Exit(1)
	}

	parseConstants(
		os.Args[1],
		os.Args[2],
		regexpStr,
		groups,
		types,
		templArgs{
			packageNameKey:  pkgName,
			repoNameKey:     repoName,
			optionalCodeKey: goGenStr,
		})
}
