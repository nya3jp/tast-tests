// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package graphics contains graphics-related utility functions for local tests.
package graphics

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

type bound struct {
	width  int
	height int
}

// Connector identifies the attributes related to display connector.
type Connector struct {
	Cid       int    // connector id
	Connected bool   // true if the connector is connected
	Name      string // name of the connector
	Encoders  []int  // encoders id
}

// DumpModetestOnError dumps the output of modetest to a file if the test failed.
func DumpModetestOnError(ctx context.Context, outDir string, hasError func() bool) {
	if !hasError() {
		return
	}
	file := filepath.Join(outDir, "modetest.txt")
	f, err := os.Create(file)
	if err != nil {
		testing.ContextLogf(ctx, "Failed to create %s: %v", file, err)
		return
	}
	defer f.Close()

	cmd := testexec.CommandContext(ctx, "modetest", "-c")
	cmd.Stdout, cmd.Stderr = f, f
	if err := cmd.Run(); err != nil {
		testing.ContextLog(ctx, "Failed to run modetest: ", err)
	}
}

// modesetConnectorPattern matches the second line of the following output:
// id      encoder status          name            size (mm)       modes   encoders
// 39      0       connected       eDP-1           256x144         1       38 34
var modesetConnectorPattern = regexp.MustCompile(`^(\d+)\s+\d+\s+(connected|disconnected)\s+(\S+)\s+\d+x\d+\s+\d+\s+(.+)$`)

// splitAndConvertInt splits string with comma and whitespace then convert each sub-string to int.
func splitAndConvertInt(input string) ([]int, error) {
	splitPattern := regexp.MustCompile(` *, *`)
	substrings := splitPattern.Split(input, -1)
	var result []int
	for _, substring := range substrings {
		i, err := strconv.Atoi(substring)
		if err != nil {
			return nil, err
		}
		result = append(result, i)
	}
	return result, nil
}

// ModetestConnectors retrieves a list of connectors using modetest.
func ModetestConnectors(ctx context.Context) ([]*Connector, error) {
	output, err := testexec.CommandContext(ctx, "modetest", "-c").Output()
	if err != nil {
		return nil, err
	}

	var connectors []*Connector
	for _, line := range strings.Split(string(output), "\n") {
		matches := modesetConnectorPattern.FindStringSubmatch(line)
		if matches != nil {
			cid, err := strconv.Atoi(matches[1])
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse cid %s", matches[1])
			}
			connected := (matches[2] == "connected")
			name := matches[3]
			encoders, err := splitAndConvertInt(matches[4])
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse encoders %s", matches[4])
			}
			connectors = append(connectors, &Connector{Cid: cid, Connected: connected, Name: name, Encoders: encoders})
			continue
		}
	}
	return connectors, nil
}

// NumberOfOutputsConnected parses the output of modetest to determine the number of connected displays.
// And returns the number of connected displays.
func NumberOfOutputsConnected(ctx context.Context) (int, error) {
	connectors, err := ModetestConnectors(ctx)
	if err != nil {
		return 0, err
	}
	connected := 0
	for _, display := range connectors {
		if display.Connected {
			connected++
		}
	}
	return connected, nil
}

// supportedFormatsPrimaryDisplayPattern matches the the following output:
// 0x00000001
//  formats: C8   RG16 XR24 XB24 AR24 AB24 XR30 XB30 YUYV YVYU UYVY VYUY NV12
//  props:
// 	7 type:
// 		flags: immutable enum
// 		enums: Overlay=0 Primary=1 Cursor=2
//		value: 1
// Specifically: we want the crtc matching 0x00...01, which is the device's internal panel (as opposed to external panels)
// We want to make sure it has formats, props, etc.;
// And we want it to be a primary display, not an overlay or a cursor (its value is 1)
var supportedFormatsPrimaryDisplayPattern = regexp.MustCompile(`0x(0+)1\n\s+formats:([ \t]+(\S+))+\n\s+props:\n([^\n]+\n){3}\s+value: 1`)

// supportedFormatsPattern matches the formats portion of the output, specifically:
//  formats: C8   RG16 XR24 XB24 AR24 AB24 XR30 XB30 YUYV YVYU UYVY VYUY NV12
var supportedFormatsPattern = regexp.MustCompile(`\s+formats:(([ \t]+\S+)+)`)

// ModetestPrimaryDisplayFormatsSupported determines the supported formats of the first primary display.
func ModetestPrimaryDisplayFormatsSupported(ctx context.Context) ([]string, error) {
	output, err := testexec.CommandContext(ctx, "modetest", "-p").Output()
	if err != nil {
		return nil, err
	}

	primaryDisplayFormatInfo := supportedFormatsPrimaryDisplayPattern.FindString(string(output))
	if primaryDisplayFormatInfo == "" {
		testing.ContextLog(ctx, "Failed to find primary display format with modetest")
		// Question: what is standard for these error messages?
		return nil, errors.New("display format parsing") //TODO error message better
	}

	formatLine := supportedFormatsPattern.FindStringSubmatch(primaryDisplayFormatInfo)
	if formatLine == nil {
		testing.ContextLogf(ctx, "Failed to find format within primary display format: %s", primaryDisplayFormatInfo)
		return nil, errors.New("really bad error in format parsing") //TODO error message better
	}
	// formats example:
	//  C8   RG16 XR24 XB24 AR24 AB24 XR30 XB30 YUYV YVYU UYVY VYUY NV12
	formats := formatLine[1]
	return strings.Fields(formats), nil
}
