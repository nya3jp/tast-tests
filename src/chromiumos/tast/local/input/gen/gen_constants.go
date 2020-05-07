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

	"chromiumos/tast/genutil"
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

	params := genutil.Params{
		PackageName: "input",
		RepoName:    "Linux kernel",
		PreludeCode: `//go:generate ` + goSh + ` run ` + thisFile + ` ../../../../../../../third_party/kernel/v4.14/include/uapi/linux/input-event-codes.h generated_constants.go
//go:generate ` + goSh + ` fmt generated_constants.go`,
		CopyrightYear:  2018,
		MainGoFilePath: thisFile,

		Types: []genutil.TypeSpec{{
			Name:       etType,
			NativeType: "uint16",
			Desc: `corresponds to the "type" field in the input_event C struct.
	// Per the kernel documentation, "event types are groupings of codes under a logical input construct."
	// Stated more plainly, event types represent broad categories like "keyboard events".`,
		}, {
			Name:       ecType,
			NativeType: "uint16",
			Desc: `corresponds to the "code" field in the input_event C struct.
	// Per the kernel documentation, "event codes define the precise type of event."
	// There are codes corresponding to different keys on a keyboard or different mouse buttons, for example.`,
		}, {
			Name:       propType,
			NativeType: "uint16",
			Desc: `describes additional information about an input device beyond
	// the event types that it supports.`,
		}},

		Groups: []genutil.GroupSpec{
			{Prefix: "EV", TypeName: etType, Desc: "Event types"},
			{Prefix: "SYN", TypeName: ecType, Desc: "Synchronization events"},
			{Prefix: "KEY", TypeName: ecType, Desc: "Keyboard events"},
			{Prefix: "BTN", TypeName: ecType, Desc: "Momentary switch events"},
			{Prefix: "REL", TypeName: ecType, Desc: "Relative change events"},
			{Prefix: "ABS", TypeName: ecType, Desc: "Absolute change events"},
			{Prefix: "SW", TypeName: ecType, Desc: "Stateful binary switch events"},
			{Prefix: "MSC", TypeName: ecType, Desc: "Miscellaneous input and output events"},
			{Prefix: "LED", TypeName: ecType, Desc: "LED events"},
			{Prefix: "SND", TypeName: ecType, Desc: "Commands to simple sound output devices"},
			{Prefix: "REP", TypeName: ecType, Desc: "Autorepeat events"},
			{Prefix: "INPUT_PROP", TypeName: propType, Desc: "Device properties"},
		},

		LineParser: func() genutil.LineParser {
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

	if err := genutil.GenerateConstants(inputFile, outputFile, params); err != nil {
		log.Fatalf("Failed to generate %v: %v", outputFile, err)
	}
}
