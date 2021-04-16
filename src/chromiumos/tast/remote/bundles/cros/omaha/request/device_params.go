// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package request

import (
	"context"
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

// GenAPPRequest generates a App request for the OS.
func (d *DeviceParams) GenAPPRequest(version string, track string) App {
	return App{
		APPID:         d.ProductID,
		Board:         d.Board,
		HardwareClass: d.HardwareID,
		DeltaOk:       true,
		Lang:          "en-US",

		Track:   track,
		Version: version,
	}
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
