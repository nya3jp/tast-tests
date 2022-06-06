// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package usbutils

import (
	"bufio"
	"context"
	"regexp"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
)

// USBDevice represents information of a connected USB device.
type USBDevice struct {
	// Class represents class that the connected device falls into. (Example: Mass storage, Wireless, etc).
	Class string
	// Driver represents driver that drives the connected device. (Example: hub, btusb, etc).
	Driver string
	// Speed represents the speed of connected device. (Example: 480M, 5000M, 1.5M, etc).
	Speed string
}

// re will parse Class, Driver and Speed of USB devices from 'lsusb -t' command output.
// Sample output of 'lsusb -t' command is as below:
/*
/:  Bus 04.Port 1: Dev 1, Class=root_hub, Driver=xhci_hcd/4p, 10000M
/:  Bus 03.Port 1: Dev 1, Class=root_hub, Driver=xhci_hcd/12p, 480M
    |__ Port 2: Dev 2, If 0, Class=Mass Storage, Driver=usb-storage, 5000M
    |__ Port 2: Dev 2, If 0, Class=Vendor Specific Class, Driver=asix, 480M
    |__ Port 5: Dev 3, If 0, Class=Video, Driver=uvcvideo, 480M
    |__ Port 5: Dev 3, If 1, Class=Video, Driver=uvcvideo, 480M
    |__ Port 10: Dev 4, If 0, Class=Wireless, Driver=btusb, 12M
    |__ Port 10: Dev 4, If 1, Class=Wireless, Driver=btusb, 12M
/:  Bus 02.Port 1: Dev 1, Class=root_hub, Driver=xhci_hcd/4p, 10000M
/:  Bus 01.Port 1: Dev 1, Class=root_hub, Driver=xhci_hcd/1p, 480M
*/
var re = regexp.MustCompile(`Class=([a-zA-Z_\s]+), Driver=([a-zA-Z0-9_\-\/\s]+), ([a-zA-Z0-9_\/.]+)`)

// ListDevicesInfo returns the class, driver and speed for all the USB devices.
//
// For local-side dut parameter must be nil.
// Example:
// usbDeviceInfo, err := usbutils.ListDevicesInfo(ctx, nil){
// ...
//
// For remote-side dut parameter must be non-nil.
// Example:
// dut := s.DUT()
// usbDeviceInfo, err := usbutils.ListDevicesInfo(ctx, dut){
// ...
func ListDevicesInfo(ctx context.Context, dut *dut.DUT) ([]USBDevice, error) {
	var out []byte
	var err error
	if dut != nil {
		out, err = dut.Conn().CommandContext(ctx, "lsusb", "-t").Output()
	} else {
		out, err = testexec.CommandContext(ctx, "lsusb", "-t").Output()
	}

	if err != nil {
		return nil, errors.Wrap(err, "failed to run lsusb command")
	}
	lsusbOut := string(out)
	var res []USBDevice
	sc := bufio.NewScanner(strings.NewReader(lsusbOut))
	for sc.Scan() {
		match := re.FindStringSubmatch(sc.Text())
		if match == nil {
			continue
		}
		res = append(res, USBDevice{
			Class:  match[1],
			Driver: match[2],
			Speed:  match[3],
		})
	}
	return res, nil
}

// NumberOfUSBDevicesConnected returns number of all USB devices connected with given devClassName, usbSpeed.
func NumberOfUSBDevicesConnected(deviceInfoList []USBDevice, devClassName, usbSpeed string) int {
	var speedSlice []string
	for _, dev := range deviceInfoList {
		if dev.Class == devClassName {
			devSpeed := dev.Speed
			if devSpeed == usbSpeed {
				speedSlice = append(speedSlice, devSpeed)
			}
		}
	}
	numberOfDevicesConnected := len(speedSlice)
	return numberOfDevicesConnected
}
