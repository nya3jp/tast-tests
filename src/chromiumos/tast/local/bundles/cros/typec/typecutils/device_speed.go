// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typecutils

import (
	"bufio"
	"context"
	"regexp"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

// Lsusb represents information of all USB devices.
type Lsusb struct {
	Class  string
	Driver string
	Speed  string
}

// ListDeviceSpeed returns the class, driver and speed for all the USB devices.
func ListDeviceSpeed(ctx context.Context) ([]Lsusb, error) {
	out, err := testexec.CommandContext(ctx, "lsusb", "-t").Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to run lsusb command")
	}
	lsusbOut := string(out)
	re := regexp.MustCompile(`.*Class=([a-zA-Z_\s]+).*Driver=([a-zA-Z0-9_\-\/\s]+).*,.([a-zA-Z0-9_\/]+)`)
	var resSlice []Lsusb
	sc := bufio.NewScanner(strings.NewReader(lsusbOut))
	for sc.Scan() {
		var l Lsusb
		match := re.FindStringSubmatch(sc.Text())
		if match == nil {
			continue
		}
		l.Class, l.Driver, l.Speed = match[1], match[2], match[3]
		resSlice = append(resSlice, l)
	}
	return resSlice, nil
}

// DeviceSpeed returns mass storage device speed for all USB devices.
// If failed to get devices speed returns error.
func DeviceSpeed(ctx context.Context) ([]string, error) {
	res, err := ListDeviceSpeed(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get lsusb details")
	}
	devSpeed := ""
	found := false
	var speedSlice []string
	for _, dev := range res {
		if dev.Class == "Mass Storage" {
			devSpeed = dev.Speed
			if devSpeed != "" {
				found = true
				speedSlice = append(speedSlice, devSpeed)
			}
		}
	}
	if !found {
		return nil, errors.New("failed to find USB device speed")
	}
	return speedSlice, nil
}
