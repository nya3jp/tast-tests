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

// connector identifies the attributes related to display connector.
type connector struct {
	cid       int    // connector id
	ctype     string // connector type, e.g. 'eDP', 'HDMI-A', 'DP'
	connected bool   // true if the connector is connected
	size      bound  // current screen size
}

var (
	modesetConnectorPattern = regexp.MustCompile(`^(\d+)\s+\d+\s+(connected|disconnected)\s+(\S+)\s+\d+x\d+\s+\d+\s+\d+`)

	// Group names match the drmModeModeInfo struct
	modesetModePattern = regexp.MustCompile(`\s+(?P<name>.+)\s+(?P<vrefresh>\d+)\s+(?P<hdisplay>\d+)\s+(?P<hsync_start>\d+)\s+(?P<hsync_end>\d+)\s+(?P<htotal>\d+)\s+(?P<vdisplay>\d+)\s+(?P<vsync_start>\d+)\s+(?P<vsync_end>\d+)\s+(?P<vtotal>\d+)\s+(?P<clock>\d+)\s+flags:.+type: preferred`)
)

// modetestConnectors retrieves a list of connectors using modetest.
func modetestConnectors(ctx context.Context) ([]*connector, error) {
	output, err := testexec.CommandContext(ctx, "modetest", "-c").Output()
	if err != nil {
		return nil, err
	}

	var connectors []*connector
	for _, line := range strings.Split(string(output), "\n") {
		matches := modesetConnectorPattern.FindStringSubmatch(line)
		if matches != nil {
			cid, err := strconv.Atoi(matches[1])
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse cid %s", matches[1])
			}
			connected := false
			if matches[2] == "connected" {
				connected = true
			}
			ctype := matches[3]
			connectors = append(connectors, &connector{cid: cid, ctype: ctype, connected: connected})
			continue
		}

		matches = modesetModePattern.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		var size bound
		var err error
		for i, name := range modesetModePattern.SubexpNames() {
			if name == "hdisplay" {
				size.width, err = strconv.Atoi(matches[i])
			} else if name == "vdisplay" {
				size.height, err = strconv.Atoi(matches[i])
			}
			if err != nil {
				return connectors, errors.Wrapf(err, "failed to parse %s", name)
			}
		}
		if len(connectors) == 0 {
			return connectors, errors.Wrap(err, "failed to update last connector")
		}
		if size.width > 0 && size.height > 0 {
			// Update the last connector in the list.
			connectors[len(connectors)-1].size = size
		}
	}
	// Check if all connected connectors have size set.
	for _, display := range connectors {
		if !display.connected {
			continue
		}
		if display.size.width <= 0 || display.size.height <= 0 {
			return nil, errors.Wrapf(err, "failed to fetch reasonable connector size: %q", display.size)
		}
	}

	return connectors, nil
}

// NumberOfOutputsConnected parses the output of modetest to determine the number of connected displays.
// And returns the number of connected displays.
func NumberOfOutputsConnected(ctx context.Context) (int, error) {
	connectors, err := modetestConnectors(ctx)
	if err != nil {
		return 0, err
	}
	connected := 0
	for _, display := range connectors {
		if display.connected {
			connected++
		}
	}
	return connected, nil
}
