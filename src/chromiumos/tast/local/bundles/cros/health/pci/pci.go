// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package pci provides PCI util functions for health tast.
package pci

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

// Device holds the result from lspci.
type Device struct {
	VendorID string
	DeviceID string
	Vendor   string
	Device   string
	Class    string
	ProgIf   string
	Driver   *string
}

// For mocking.
var lspciCmd = func(ctx context.Context, arg string) ([]byte, error) {
	return testexec.CommandContext(ctx, "lspci", "-vmmk", arg).Output(testexec.DumpLogOnError)
}

// lspci runs `lspci -vmmk` and returns the attributes of each device, which is
// represented by a list of key-value map.
// The output is `key: value` on each line. Each device is ended by an empty
// line.
func lspci(ctx context.Context, arg string) ([]map[string]string, error) {
	b, err := lspciCmd(ctx, arg)
	if err != nil {
		return nil, err
	}
	sc := bufio.NewScanner(strings.NewReader(string(b)))
	var res []map[string]string
	dev := make(map[string]string)
	for sc.Scan() {
		// Each line should be like "Key:   value   ". Otherwise, it should be an
		// empty line which terminates a device.
		tokens := strings.SplitN(sc.Text(), ":", 2)
		if len(tokens) == 2 {
			k, v := tokens[0], strings.TrimSpace(tokens[1])
			dev[k] = v
		} else {
			res = append(res, dev)
			dev = make(map[string]string)
		}
	}
	return res, nil
}

// ExpectedDevices returns expected PCI devices, sorted by VendorID + DeviceID.
func ExpectedDevices(ctx context.Context) ([]Device, error) {
	const pciPath = "/sys/bus/pci"
	// Check if PCI is supported or not before calling lspci.
	if _, err := os.Stat(pciPath); os.IsNotExist(err) {
		return nil, nil
	}

	// "-n" ask lspci to output the hex value of each field.
	devs, err := lspci(ctx, "-n")
	if err != nil {
		return nil, err
	}

	var res []Device
	for _, d := range devs {
		r := Device{
			VendorID: d["Vendor"],
			DeviceID: d["Device"],
			Class:    d["Class"],
			ProgIf:   "00",
		}
		if v, ok := d["ProgIf"]; ok {
			r.ProgIf = v
		}
		if v, ok := d["Driver"]; ok {
			r.Driver = &v
		}
		if len(r.VendorID) == 0 {
			return nil, errors.Errorf("cannot get vendor id, got: %v", d)
		}
		if len(r.DeviceID) == 0 {
			return nil, errors.Errorf("cannot get device id, got: %v", d)
		}
		// Get the string value of each field of the device with VendorID:DeviceID.
		arg := fmt.Sprintf("-d%s:%s", r.VendorID, r.DeviceID)
		devsStr, err := lspci(ctx, arg)
		if err != nil {
			return nil, err
		}
		if len(devsStr) == 0 {
			return nil, errors.Errorf("cannot find device: %v", arg)
		}
		dStr := devsStr[0]
		r.Vendor = dStr["Vendor"]
		r.Device = dStr["Device"]
		res = append(res, r)
	}
	Sort(res)
	return res, nil
}

// keys returns the keys to sort a device.
func (d *Device) keys() []string {
	dr := "(none)"
	if d.Driver != nil {
		dr = *d.Driver
	}
	return []string{
		d.VendorID,
		d.DeviceID,
		d.Vendor,
		d.Device,
		d.Class,
		d.ProgIf,
		dr,
	}
}

// Sort sorts a slice of Devices.
func Sort(d []Device) {
	sort.Slice(d, func(i, j int) bool {
		x := d[i].keys()
		y := d[j].keys()
		if len(x) != len(y) {
			return len(x) < len(y)
		}
		for i := range x {
			if x[i] != y[i] {
				return x[i] < y[i]
			}
		}
		return false
	})
}
