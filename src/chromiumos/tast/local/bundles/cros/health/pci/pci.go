// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package pci provides pci util functions for health tast.
package pci

import (
	"bufio"
	"context"
	"fmt"
	"sort"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

// Device hold the result from lspci.
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
var lspciCmd = func(ctx context.Context, args ...string) ([]byte, error) {
	return testexec.CommandContext(ctx, "lspci", args...).Output(testexec.DumpLogOnError)
}

func lspci(ctx context.Context, args ...string) ([]map[string]string, error) {
	args = append([]string{"-vmmk"}, args...)
	b, err := lspciCmd(ctx, args...)
	if err != nil {
		return nil, err
	}
	d := make(map[string]string)
	var res []map[string]string
	sc := bufio.NewScanner(strings.NewReader(string(b)))
	for sc.Scan() {
		tokens := strings.SplitN(sc.Text(), ":", 2)
		if len(tokens) == 2 {
			d[tokens[0]] = strings.TrimSpace(tokens[1])
		} else {
			res = append(res, d)
			d = make(map[string]string)
		}
	}
	return res, nil
}

// ExpectedDevices returns expected pci devices.
func ExpectedDevices(ctx context.Context) ([]Device, error) {
	var res []Device
	devs, err := lspci(ctx, "-n")
	if err != nil {
		return nil, err
	}
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
	Sorted(res)
	return res, nil
}

// Sorted sorts a slice of Device and return it.
func Sorted(d []Device) {
	sort.Slice(d, func(i, j int) bool {
		x := d[i]
		y := d[j]
		return x.VendorID+x.DeviceID < y.VendorID+y.DeviceID
	})
}
