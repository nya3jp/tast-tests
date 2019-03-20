// Copyright 2018 The Chromium OS Authors. All rights reserved.
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
	pkgName = "input"

	regexpStr = `^#define\s+([A-Z][_A-Z0-9]+)\s+(0x[0-9a-fA-F]+|\d+)`

	// The Git repository from where the constants were taken from.
	repoName = "Linux kernel"

	// Type names of generated constants.
	etType   = "EventType"
	ecType   = "EventCode"
	propType = "DeviceProperty"
)

var types []*typeInfo = []*typeInfo{
	&typeInfo{
		etType,
		"uint16",
		`corresponds to the "type" field in the input_event C struct.
// Per the kernel documentation, "event types are groupings of codes under a logical input construct."
// Stated more plainly, event types represent broad categories like "keyboard events".`,
	},
	&typeInfo{
		ecType,
		"uint16",
		`corresponds to the "code" field in the input_event C struct.
// Per the kernel documentation, "event codes define the precise type of event."
// There are codes corresponding to different keys on a keyboard or different mouse buttons, for example.`,
	},
	&typeInfo{
		propType,
		"uint16",
		`describes additional information about an input device beyond
// the event types that it supports.`,
	},
}

// These are documented at https://www.kernel.org/doc/Documentation/input/event-codes.txt.
var groups []*groupInfo = []*groupInfo{
	&groupInfo{"EV", etType, "Event types"},
	&groupInfo{"SYN", ecType, "Synchronization events"},
	&groupInfo{"KEY", ecType, "Keyboard events"},
	&groupInfo{"BTN", ecType, "Momentary switch events"},
	&groupInfo{"REL", ecType, "Relative change events"},
	&groupInfo{"ABS", ecType, "Absolute change events"},
	&groupInfo{"SW", ecType, "Stateful binary switch events"},
	&groupInfo{"MSC", ecType, "Miscellaneous input and output events"},
	&groupInfo{"LED", ecType, "LED events"},
	&groupInfo{"SND", ecType, "Commands to simple sound output devices"},
	&groupInfo{"REP", ecType, "Autorepeat events"},
	&groupInfo{"INPUT_PROP", propType, "Device properties"},
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
			packageNameKey: pkgName,
			repoNameKey:    repoName,
		})
}
