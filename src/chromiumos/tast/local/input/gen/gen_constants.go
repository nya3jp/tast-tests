// Copyright 2018 The Chromium OS Authors. All rights reserved.
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

	"github.com/pkg/errors"
)

// readConstants reads path, a kernel input-event-codes.h file, and returns a subset of relevant constants from it.
// groups represents the map the contains the different groups.
func readConstants(groups []*groupInfo, path string) (constantGroups, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	consts := make(constantGroups)
	re := regexp.MustCompile(`^#define\s+([A-Z][_A-Z0-9]+)\s+(0x[0-9a-fA-F]+|\d+)`)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		matches := re.FindStringSubmatch(sc.Text())
		if matches == nil {
			continue
		}
		name, sval := matches[1], matches[2]
		grp := getGroupForName(groups, name)
		if grp == nil {
			return nil, errors.Errorf("unable to classify %q", name)
		} else if name == grp.prefix+"_MAX" {
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
		fmt.Fprintf(os.Stderr, "Usage: %s <input-event-codes.h> <out.go>\n", os.Args[0])
		os.Exit(1)
	}

	inputFile := args[0]
	outputFile := args[1]

	repoPath, repoRev, err := getRepoInfo(inputFile)
	if err != nil {
		log.Fatalf("Failed to get repo info for %v: %v", inputFile, err)
	}

	const (
		etType   = "EventType"
		ecType   = "EventCode"
		propType = "DeviceProperty"
	)

	var types = []*typeInfo{
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
	var groups = []*groupInfo{
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

	const (
		exeName  = "gen/gen_constants.go"
		repoName = "Linux kernel"
		pkgName  = "input"
		goGen    = `//go:generate go run ` + exeName + ` gen/util.go ../../../../../../../third_party/kernel/v4.14/include/uapi/linux/input-event-codes.h generated_constants.go
//go:generate go fmt generated_constants.go`
	)

	a := tmplArgs{
		RepoPath:       repoPath,
		RepoRev:        repoRev,
		RepoName:       repoName,
		PackageName:    pkgName,
		PreludeCode:    goGen,
		ExecutableName: exeName,
		CopyrightYear:  "2018",
	}

	consts, err := readConstants(groups, inputFile)
	if err != nil {
		log.Fatalf("Failed to read %v: %v", inputFile, err)
	}

	if err := writeConstants(consts, groups, types, a, outputFile); err != nil {
		log.Fatalf("Failed to write %v: %v", outputFile, err)
	}
}
