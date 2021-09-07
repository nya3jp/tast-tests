// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package usb provides usb util functions for health tast.
package usb

import (
	"bufio"
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

// Device hold the result from usb-devices.
type Device struct {
	VendorID   string
	ProdID     string
	DeviceName string
	Class      string
	SubClass   string
	Protocol   string
	Interfaces []Interface
}

// Interface hold the result from usb-devices.
type Interface struct {
	InterfaceNumber string
	Class           string
	SubClass        string
	Protocol        string
	Driver          *string
}

// For mocking.
var runCommand = func(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	return testexec.CommandContext(ctx, cmd, args...).Output(testexec.DumpLogOnError)
}

// usbDevices returns a list of device. Each device is represented as a list of
// string. Each string contains some attribute related to the device.
func usbDevices(ctx context.Context) ([][]string, error) {
	b, err := runCommand(ctx, "usb-devices")
	if err != nil {
		return nil, err
	}
	var res [][]string
	sc := bufio.NewScanner(strings.NewReader(string(b)))
	for sc.Scan() {
		if sc.Text() == "" {
			// Each device is started by an empty line.
			res = append(res, []string{})
		} else {
			i := len(res) - 1
			res[i] = append(res[i], sc.Text())
		}
	}
	return res, nil
}

// getDeviceName returns the string name of device with vendorID:prodID. The
// name is extracted from lsusb.
func getDeviceName(ctx context.Context, vendorID, prodID string) (string, error) {
	arg := fmt.Sprintf("-d%s:%s", vendorID, prodID)
	b, err := runCommand(ctx, "lsusb", arg)
	if err != nil {
		return "", err
	}
	// The name is the suffix string after ID field.
	re := regexp.MustCompile(`^Bus [0-9]+ Device [0-9]+: ID [0-9a-f]+:[0-9a-f]+ ([^\n]+)\n`)
	m := re.FindStringSubmatch(string(b))
	if m == nil {
		return "", errors.Errorf("failed to parse lsusb output: %v", string(b))
	}
	return m[1], nil
}

// ExpectedDevices returns expected usb devices, sorted by VendorID+ProdID.
func ExpectedDevices(ctx context.Context) ([]Device, error) {
	// Reference: https://www.kernel.org/doc/html/v4.12/driver-api/usb/usb.html#sys-kernel-debug-usb-devices-output-format
	reD := regexp.MustCompile(`Cls=([0-9a-f]{2}).* Sub=([0-9a-f]{2}) Prot=([0-9a-f]{2})`)
	reP := regexp.MustCompile(`Vendor=([0-9a-f]{4}) ProdID=([0-9a-f]{4})`)
	reI := regexp.MustCompile(`If#=0x([0-9a-f]+) .* Cls=([0-9a-f]{2}).* Sub=([0-9a-f]{2}) Prot=([0-9a-f]{2}) Driver=([\S]*)`)

	var res []Device
	devs, err := usbDevices(ctx)
	if err != nil {
		return nil, err
	}
	for _, d := range devs {
		var r Device
		for _, l := range d {
			switch l[0] {
			case 'D':
				m := reD.FindStringSubmatch(l)
				if m == nil {
					return nil, errors.Errorf("cannot parse usb-devices D: %v", l)
				}
				r.Class, r.SubClass, r.Protocol = m[1], m[2], m[3]
			case 'P':
				m := reP.FindStringSubmatch(l)
				if m == nil {
					return nil, errors.Errorf("cannot parse usb-devices P: %v", l)
				}
				r.VendorID, r.ProdID = m[1], m[2]
			case 'I':
				m := reI.FindStringSubmatch(l)
				if m == nil {
					return nil, errors.Errorf("cannot parse usb-devices I: %v", l)
				}
				ifc := Interface{
					InterfaceNumber: m[1],
					Class:           m[2],
					SubClass:        m[3],
					Protocol:        m[4],
					Driver:          &m[5],
				}
				if *ifc.Driver == "(none)" {
					ifc.Driver = nil
				}
				r.Interfaces = append(r.Interfaces, ifc)
			}
		}
		var err error
		if r.DeviceName, err = getDeviceName(ctx, r.VendorID, r.ProdID); err != nil {
			return nil, err
		}
		res = append(res, r)
	}
	Sort(res)
	return res, nil
}

// Sort sorts a slice of Device by VendorID+ProdID.
func Sort(d []Device) {
	sort.Slice(d, func(i, j int) bool {
		x := d[i]
		y := d[j]
		return x.VendorID+x.ProdID < y.VendorID+y.ProdID
	})
}
