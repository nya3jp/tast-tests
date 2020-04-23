// Copyright 2018 The Chromium OS Authors. All rights reserved.
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
)

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <input-event-codes.h> <out.go>\n", os.Args[0])
		os.Exit(1)
	}

	inputFile := args[0]
	outputFile := args[1]

	const (
		// Relative path to go.sh from the generated file.
		goSh = "../../../../../../tast/tools/go.sh"

		// Relative path to this file from the generated file.
		thisFile = "gen/gen_constants.go"

		etType   = "EventType"
		ecType   = "EventCode"
		propType = "DeviceProperty"
	)

	params := Params{
		PackageName: "input",
		RepoName:    "Linux kernel",
		PreludeCode: `//go:generate ` + goSh + ` run ` + thisFile + ` gen/util.go ../../../../../../../third_party/kernel/v4.14/include/uapi/linux/input-event-codes.h generated_constants.go
//go:generate ` + goSh + ` fmt generated_constants.go`,
		CopyrightYear:  2018,
		MainGoFilePath: thisFile,

		Types: []TypeSpec{{
			etType,
			"uint16",
			`corresponds to the "type" field in the input_event C struct.
	// Per the kernel documentation, "event types are groupings of codes under a logical input construct."
	// Stated more plainly, event types represent broad categories like "keyboard events".`,
		}, {
			ecType,
			"uint16",
			`corresponds to the "code" field in the input_event C struct.
	// Per the kernel documentation, "event codes define the precise type of event."
	// There are codes corresponding to different keys on a keyboard or different mouse buttons, for example.`,
		}, {
			propType,
			"uint16",
			`describes additional information about an input device beyond
	// the event types that it supports.`,
		}},

		Groups: []GroupSpec{
			{"EV", etType, "Event types"},
			{"SYN", ecType, "Synchronization events"},
			{"KEY", ecType, "Keyboard events"},
			{"BTN", ecType, "Momentary switch events"},
			{"REL", ecType, "Relative change events"},
			{"ABS", ecType, "Absolute change events"},
			{"SW", ecType, "Stateful binary switch events"},
			{"MSC", ecType, "Miscellaneous input and output events"},
			{"LED", ecType, "LED events"},
			{"SND", ecType, "Commands to simple sound output devices"},
			{"REP", ecType, "Autorepeat events"},
			{"INPUT_PROP", propType, "Device properties"},
		},

		LineParser: func() LineParser {
			// Reads inputFile, a kernel input-event-codes.h. Looking for lines like:
			//   #define EV_SYN 0x00
			re := regexp.MustCompile(`^#define\s+([A-Z][_A-Z0-9]+)\s+(0x[0-9a-fA-F]+|\d+)`)
			return func(line string) (name, sval string, ok bool) {
				m := re.FindStringSubmatch(line)
				if m == nil {
					return "", "", false
				}
				return m[1], m[2], true
			}
		}(),
	}

	if err := GenerateConstants(inputFile, outputFile, params); err != nil {
		log.Fatalf("Failed to generate %v: %v", outputFile, err)
	}
}
