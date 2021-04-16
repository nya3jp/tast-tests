// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package request

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"strings"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/lsbrelease"
)

// DeviceParams contains the device specific parameters used by update_engine.
type DeviceParams struct {
	Board       string
	ProductID   string
	MachineType string
	HardwareID  string
}

// GenSP generates a string to be used in the SP field of OS.
func (d *DeviceParams) GenSP(version string) string {
	return version + "_" + d.MachineType
}

// GenAPPRequest generates a RequestApp for the OS.
func (d *DeviceParams) GenAPPRequest(version, track string) RequestApp {
	return RequestApp{
		APPID:         d.ProductID,
		Board:         d.Board,
		HardwareClass: d.HardwareID,
		DeltaOk:       true,
		Lang:          "en-US",

		Track:   track,
		Version: version,
	}
}

// DumpToFile writes the device parameters to a file.
func (d *DeviceParams) DumpToFile(path string) error {
	file, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal device params")
	}

	return ioutil.WriteFile(path, file, 0644)
}

// LoadParamsFromDUT reads the Omaha configuration for the device from the DUT.
func LoadParamsFromDUT(ctx context.Context, d *dut.DUT) (*DeviceParams, error) {
	lsbContents, err := d.Conn().Command("cat", "/etc/lsb-release").Output(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "loading lsbrelease contents")
	}
	lsbMap, err := lsbrelease.Parse(strings.NewReader(string(lsbContents)))
	if err != nil {
		return nil, errors.Wrap(err, "parsing lsbrelease contents")
	}

	board := ""
	if tmp, ok := lsbMap[lsbrelease.Board]; ok {
		board = tmp
	}

	// Test images don't have the -signed suffix, add it here to simulate a normal image.
	board = board + "-signed-omahatest"

	productID := DefaultAppID
	if tmp, ok := lsbMap[lsbrelease.ReleaseAppID]; ok {
		productID = tmp
	}
	if tmp, ok := lsbMap[lsbrelease.BoardAppID]; ok {
		productID = tmp
	}

	machineTypeRaw, err := d.Conn().Command("uname", "-m").Output(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "uname failed")
	}

	machineType := strings.TrimSpace(string(machineTypeRaw))

	hardwareIDRaw, err := d.Conn().Command("crossystem", "hwid").Output(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "crossystem hwid failed")
	}

	hardwareID := strings.TrimSpace(string(hardwareIDRaw))

	return &DeviceParams{
		Board:       board,
		ProductID:   productID,
		MachineType: machineType,
		HardwareID:  hardwareID,
	}, nil
}
