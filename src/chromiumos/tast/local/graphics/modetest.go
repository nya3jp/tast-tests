// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package graphics contains graphics-related utility functions for local tests.
package graphics

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
)

type bound struct {
	width  int
	height int
}

// Connector identifies the attributes related to display connector.
type Connector struct {
	cid       int    // connector id
	Connected bool   // true if the connector is connected
	Name      string // name of the connector
	Encoders  []int  // encoders id
}

// id      encoder status          name            size (mm)       modes   encoders
// 39      0       connected       eDP-1           256x144         1       38, 34
var modesetConnectorPattern = regexp.MustCompile(`^(\d+)\s+\d+\s+(connected|disconnected)\s+(\S+)\s+\d+x\d+\s+\d+\s+(.+)$`)

// ModetestConnectors retrieves a list of connectors using modetest.
func ModetestConnectors(ctx context.Context) ([]*Connector, error) {
	output, err := testexec.CommandContext(ctx, "modetest", "-c").Output()
	if err != nil {
		return nil, err
	}

	// split string with white space and convert each sub-string to int.
	var splitAndConvertInt = func(input string) ([]int, error) {
		substrings := strings.Fields(input)
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
			connectors = append(connectors, &Connector{cid: cid, Connected: connected, Name: name, Encoders: encoders})
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
