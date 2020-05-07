// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package main implements a script for writing a Go source file containing input event constants.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"

	"chromiumos/tast/genutil"
)

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <KeyEvent.java> <out.go>\n", os.Args[0])
		os.Exit(1)
	}

	inputFile := args[0]
	outputFile := args[1]

	const (
		// Relative path to go.sh from the generated file.
		goSh = "../../../../../../../tast/tools/go.sh"

		// Relative path to this file from the generated file.
		thisFile = "gen/gen_constants.go"

		keyCodeType   = "KeyCode"
		metaStateType = "MetaState"
	)

	params := genutil.Params{
		PackageName: "ui",
		RepoName:    "Android frameworks/base",
		PreludeCode: `// Assumes that Android repo is checked out at same folder level as Chrome OS. e.g: If Chrome OS sources are in:
// ~/src/chromeos/, then Android sources should be in ~/src/android/
//go:generate ` + goSh + ` run ` + thisFile + ` ../../../../../../../../../../android/frameworks/base/core/java/android/view/KeyEvent.java generated_constants.go
//go:generate ` + goSh + ` fmt generated_constants.go`,
		CopyrightYear:  2019,
		MainGoFilePath: thisFile,

		Types: []genutil.TypeSpec{
			{keyCodeType, "uint16", "represents an Android key code."},
			{metaStateType, "uint64", "represents a meta-key state. Each bit set to 1 represents a pressed meta key."},
		},

		// We only care about KEYCODE and META prefixes. We ignore the rest.
		Groups: []genutil.GroupSpec{
			{"KEYCODE", keyCodeType, "KeyCodes constants"},
			{"META", metaStateType, "Meta-key constants"},
		},

		// Read inputFile, a KeyEvent.java. Looking for lines like:
		//   public static final int META_SELECTING = 0x800;
		// TODO(ricardoq): Multiline, bitwise-or metas are not supported. Find a robust way to support them. e.g:
		//   public static final int META_SHIFT_MASK = META_SHIFT_ON
		//        | META_SHIFT_LEFT_ON | META_SHIFT_RIGHT_ON;
		LineParser: func() genutil.LineParser {
			re := regexp.MustCompile(`^\s+public static final int ([_A-Z0-9]+)\s*=\s*(0x[0-9a-fA-F]+|\d+);$`)
			return func(line string) (name, sval string, ok bool) {
				m := re.FindStringSubmatch(line)
				if m == nil {
					return "", "", false
				}
				return m[1], m[2], true
			}
		}(),
	}

	if err := genutil.GenerateConstants(inputFile, outputFile, params); err != nil {
		log.Fatalf("Failed to generate %v: %v", outputFile, err)
	}
}
