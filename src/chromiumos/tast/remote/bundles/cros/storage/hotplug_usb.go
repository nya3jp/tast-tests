// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package storage

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/dut"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: HotplugUSB,
		Desc: "Hotplug USB device through servo hub and verifies the detection",
		Contacts: []string{
			"kasaiah.bogineni@intel.com",
			"ningappa.tirakannavar@intel.com",
		},
		Attr: []string{"disabled", "informational"},
	})
}

// HotplugUSB device through servo hub and verifies the detection
// Need to connect usb device to Servo USB_KEY port
func HotplugUSB(ctx context.Context, s *testing.State) {
	d, ok := dut.FromContext(ctx)
	if !ok {
		s.Fatal("Failed to get DUT")
	}
	defer d.Close(ctx)
	svo, err := servo.Default(ctx)
	if err != nil {
		s.Fatal("Failed to intialize servo: ", err)
	}
	s.Log("Turn off usb key")
	if err := svo.SwitchUSBKeyPower(ctx, "off"); err != nil {
		s.Fatal("Failed to turn off usb key: ", err)
	}
	testing.Sleep(ctx, 2*time.Second) // Added 2 secs delay between SwitchUSBKeyPower and SwitchUSBKey
	getUSBDevices := func() ([]string, error) {
		devices := []string{}
		output, err := d.Run(ctx, "lsusb")
		if err != nil {
			return devices, err
		}
		// Sample lsusb output is : Bus 002 Device 003: ID 0bda:8153 Realtek Semiconductor Corp.
		for _, device := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			devices = append(devices, strings.Split(device, " ")[5])
		}
		return devices, nil
	}
	devsBeforePlug, err := getUSBDevices()
	if err != nil {
		s.Fatal("Failed to get usb devices: ", err)
	}
	s.Log("USB devices before plug: ", devsBeforePlug)
	s.Log("Hot plug usb device")
	if err := svo.SwitchUSBKey(ctx, "dut"); err != nil {
		s.Fatal("Failed to switch usb key: ", err)
	}
	devsAfterPlug, err := getUSBDevices()
	if err != nil {
		s.Fatal("Failed to get usb devices: ", err)
	}
	s.Log("USB devices after plug: ", devsAfterPlug)
	if len(devsAfterPlug) == len(devsBeforePlug) {
		s.Fatal("Hot plug unsuccessful, no new usb found")
	}
}
