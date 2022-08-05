// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package params

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"strings"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/lsbrelease"
)

// Device contains the device specific parameters used by update_engine.
type Device struct {
	Board       string
	RawBoard    string
	ProductID   string
	MachineType string
	HardwareID  string
}

// DefaultAppID is used if no device specific app id is found.
const DefaultAppID = "{87efface-864d-49a5-9bb3-4b050a7c227a}"

// DumpToFile writes the device parameters to a file.
func (d *Device) DumpToFile(path string) error {
	file, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal device params")
	}

	return ioutil.WriteFile(path, file, 0644)
}

// loadParamsFromDUT reads the Omaha configuration for the device from the DUT.
func loadParamsFromDUT(ctx context.Context, d *dut.DUT) (*Device, error) {
	lsbContents, err := d.Conn().CommandContext(ctx, "cat", "/etc/lsb-release").Output()
	if err != nil {
		return nil, errors.Wrap(err, "loading lsbrelease contents")
	}
	lsbMap, err := lsbrelease.Parse(strings.NewReader(string(lsbContents)))
	if err != nil {
		return nil, errors.Wrap(err, "parsing lsbrelease contents")
	}

	rawboard := ""
	if tmp, ok := lsbMap[lsbrelease.Board]; ok {
		rawboard = tmp
	}

	// Test images don't have the -signed suffix, add it here to simulate a normal image.
	board := rawboard + "-signed-omahatest"

	productID := DefaultAppID
	if tmp, ok := lsbMap[lsbrelease.ReleaseAppID]; ok {
		productID = tmp
	}
	if tmp, ok := lsbMap[lsbrelease.BoardAppID]; ok {
		productID = tmp
	}

	machineTypeRaw, err := d.Conn().CommandContext(ctx, "uname", "-m").Output()
	if err != nil {
		return nil, errors.Wrap(err, "uname failed")
	}

	machineType := strings.TrimSpace(string(machineTypeRaw))

	hardwareIDRaw, err := d.Conn().CommandContext(ctx, "crossystem", "hwid").Output()
	if err != nil {
		return nil, errors.Wrap(err, "crossystem hwid failed")
	}

	hardwareID := strings.TrimSpace(string(hardwareIDRaw))

	return &Device{
		Board:       board,
		RawBoard : rawboard,
		ProductID:   productID,
		MachineType: machineType,
		HardwareID:  hardwareID,
	}, nil
}
