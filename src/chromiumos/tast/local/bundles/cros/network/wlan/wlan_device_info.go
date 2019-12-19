// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wlan provides the information of the wlan device.
package wlan

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/shill"
)

// WLAN device names
const (
	marvell88w8797SDIO         = "Marvell 88W8797 SDIO"
	marvell88w8887SDIO         = "Marvell 88W8887 SDIO"
	marvell88w8897SDIO         = "Marvell 88W8897 SDIO"
	marvell88w8897PCIE         = "Marvell 88W8897 PCIE"
	marvell88w8997PCIE         = "Marvell 88W8997 PCIE"
	atherosAR9280              = "Atheros AR9280"
	atherosAR9382              = "Atheros AR9382"
	atherosAR9462              = "Atheros AR9462"
	qualcommAtherosQCA6174     = "Qualcomm Atheros QCA6174"
	qualcommAtherosQCA6174SDIO = "Qualcomm Atheros QCA6174 SDIO"
	qualcommWCN3990            = "Qualcomm WCN3990"
	intel7260                  = "Intel 7260"
	intel7265                  = "Intel 7265"
	intel9000                  = "Intel 9000"
	intel9260                  = "Intel 9260"
	intel22260                 = "Intel 22260"
	intel22560                 = "Intel 22560"
	broadcomBCM4354SDIO        = "Broadcom BCM4354 SDIO"
	broadcomBCM4356PCIE        = "Broadcom BCM4356 PCIE"
	broadcomBCM4371PCIE        = "Broadcom BCM4371 PCIE"
	realtek8822CPCIE           = "Realtek 8822C PCIE"
)

var lookupWLANDev = map[DevInfo]string{
	{Vendor: "0x02df", Device: "0x9129"}: marvell88w8797SDIO,
	{Vendor: "0x02df", Device: "0x912d"}: marvell88w8897SDIO,
	{Vendor: "0x02df", Device: "0x9135"}: marvell88w8887SDIO,
	{Vendor: "0x11ab", Device: "0x2b38"}: marvell88w8897PCIE,
	{Vendor: "0x1b4b", Device: "0x2b42"}: marvell88w8997PCIE,
	{Vendor: "0x168c", Device: "0x002a"}: atherosAR9280,
	{Vendor: "0x168c", Device: "0x0030"}: atherosAR9382,
	{Vendor: "0x168c", Device: "0x0034"}: atherosAR9462,
	{Vendor: "0x168c", Device: "0x003e"}: qualcommAtherosQCA6174,
	{Vendor: "0x105b", Device: "0xe09d"}: qualcommAtherosQCA6174,
	{Vendor: "0x0271", Device: "0x050a"}: qualcommAtherosQCA6174SDIO,
	{Vendor: "0x8086", Device: "0x08b1"}: intel7260,
	{Vendor: "0x8086", Device: "0x08b2"}: intel7260,
	{Vendor: "0x8086", Device: "0x095a"}: intel7265,
	{Vendor: "0x8086", Device: "0x095b"}: intel7265,
	// Note that Intel 9000 is also Intel 9560 aka Jefferson Peak 2
	{Vendor: "0x8086", Device: "0x9df0"}: intel9000,
	{Vendor: "0x8086", Device: "0x31dc"}: intel9000,
	{Vendor: "0x8086", Device: "0x2526"}: intel9260,
	{Vendor: "0x8086", Device: "0x2723"}: intel22260,
	// For integrated wifi chips, use device_id and subsystem_id together
	// as an identifier.
	// 0x02f0 is for Quasar on CML, 0x4070 and 0x0074 is for HrP2
	{Vendor: "0x8086", Device: "0x02f0", Subsystem: "0x0034"}: intel9000,
	{Vendor: "0x8086", Device: "0x02f0", Subsystem: "0x4070"}: intel22560,
	{Vendor: "0x8086", Device: "0x02f0", Subsystem: "0x0074"}: intel22560,
	{Vendor: "0x02d0", Device: "0x4354"}:                      broadcomBCM4354SDIO,
	{Vendor: "0x14e4", Device: "0x43ec"}:                      broadcomBCM4356PCIE,
	{Vendor: "0x14e4", Device: "0x440d"}:                      broadcomBCM4371PCIE,
	{Vendor: "0x10ec", Device: "0xc822"}:                      realtek8822CPCIE,
	{Compatible: "qcom,wcn3990-wifi"}:                         qualcommWCN3990,
}

// DevInfo contains the information of the WLAN device.
type DevInfo struct {
	// vendor is the vendor ID seen in /sys/class/net/<interface>/vendor .
	Vendor string
	// device is the product ID seen in /sys/class/net/<interface>/device .
	Device string
	// compatible is the compatible property.
	// See https://www.kernel.org/doc/Documentation/devicetree/usage-model.txt .
	Compatible string
	// subsystem is the RF chip's ID. This addition of this property is necessary for
	// device disambiguation (b/129489799).
	Subsystem string
	// The device name.
	Name string
}

// GetWLANDevInfo returns the struct DevInfo that contains the information of the WLAN device.
func GetWLANDevInfo(ctx context.Context) (*DevInfo, error) {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed creating shill manager proxy")
	}

	// GetWifiInterface returns the wireless device interface name (e.g. wlan0), or returns an error on failure.
	netIf, err := shill.GetWifiInterface(ctx, manager, 5*time.Second)
	if err != nil {
		return nil, errors.Wrap(err, "could not get a WiFi interface")
	}

	dev, err := deviceInfo(ctx, netIf)
	if err != nil {
		return nil, err
	}

	return dev, nil
}

var compatibleRE = regexp.MustCompile("^OF_COMPATIBLE_[0-9]+$")

// deviceInfo returns the device info (vendor, device, compatible and name)of the given wireless network interface
// , or returns an error on failure.
func deviceInfo(ctx context.Context, netIf string) (*DevInfo, error) {
	devicePath := filepath.Join("/sys/class/net", netIf, "device")

	dev := new(DevInfo)

	readInfo := func(x string) (string, error) {
		bs, err := ioutil.ReadFile(filepath.Join(devicePath, x))
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(bs)), nil
	}

	uevent, err := readInfo("uevent")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get uevent at device %q", netIf)
	}

	for _, line := range strings.Split(uevent, "\n") {
		kv := strings.SplitN(line, "=", 2)
		if compatibleRE.MatchString(kv[0]) {
			if d, ok := lookupWLANDev[DevInfo{Compatible: kv[1]}]; ok {
				// Found the matching device.
				dev.Name = d
				return dev, nil
			}
		}
	}

	vendorID, err := readInfo("vendor")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get vendor ID at device %q", netIf)
	}

	productID, err := readInfo("device")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get product ID at device %q", netIf)
	}
	// DUTs that use SDIO as the bus technology may not have subsystem_device at all.
	// If this is the case, just use an ID of empty string instead.
	subsystemID, err := readInfo("subsystem_device")
	if err != nil && !os.IsNotExist(err) {
		return nil, errors.Wrapf(err, "failed to get subsystem ID at device %q", netIf)
	}

	if d, ok := lookupWLANDev[DevInfo{Vendor: vendorID, Device: productID, Subsystem: subsystemID}]; ok {
		dev.Vendor = vendorID
		dev.Device = productID
		dev.Subsystem = subsystemID
		dev.Name = d
		return dev, nil
	}

	if d, ok := lookupWLANDev[DevInfo{Vendor: vendorID, Device: productID}]; ok {
		dev.Vendor = vendorID
		dev.Device = productID
		dev.Subsystem = subsystemID
		dev.Name = d
		return dev, nil
	}

	return nil, errors.Errorf("get device %s: device unknown", netIf)
}
