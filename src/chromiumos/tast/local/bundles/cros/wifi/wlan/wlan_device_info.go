// Copyright 2020 The Chromium OS Authors. All rights reserved.
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

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// WLAN device names
const (
	Marvell88w8897SDIO         = "Marvell 88W8897 SDIO"
	Marvell88w8997PCIE         = "Marvell 88W8997 PCIE"
	QualcommAtherosQCA6174     = "Qualcomm Atheros QCA6174"
	QualcommAtherosQCA6174SDIO = "Qualcomm Atheros QCA6174 SDIO"
	QualcommWCN3990            = "Qualcomm WCN3990"
	QualcommWCN6750            = "Qualcomm WCN6750"
	QualcommWCN6855            = "Qualcomm WCN6855"
	Intel7260                  = "Intel 7260"
	Intel7265                  = "Intel 7265"
	Intel9000                  = "Intel 9000"
	Intel9260                  = "Intel 9260"
	Intel22260                 = "Intel 22260"
	Intel22560                 = "Intel 22560"
	IntelAX211                 = "Intel AX 211"
	BroadcomBCM4354SDIO        = "Broadcom BCM4354 SDIO"
	BroadcomBCM4356PCIE        = "Broadcom BCM4356 PCIE"
	BroadcomBCM4371PCIE        = "Broadcom BCM4371 PCIE"
	Realtek8822CPCIE           = "Realtek 8822C PCIE"
	Realtek8852APCIE           = "Realtek 8852A PCIE"
	MediaTekMT7921PCIE         = "MediaTek MT7921 PCIE"
	MediaTekMT7921SDIO         = "MediaTek MT7921 SDIO"
	// These constants are used in the function "checkBandwidthSupport".
	intelVendorNum   = "0x8086"
	support160MHz    = '0'
	supportOnly80MHz = '2'
)

var lookupWLANDev = map[DevInfo]string{
	{vendor: "0x02df", device: "0x912d"}: Marvell88w8897SDIO,
	{vendor: "0x1b4b", device: "0x2b42"}: Marvell88w8997PCIE,
	{vendor: "0x168c", device: "0x003e"}: QualcommAtherosQCA6174,
	{vendor: "0x105b", device: "0xe09d"}: QualcommAtherosQCA6174,
	{vendor: "0x0271", device: "0x050a"}: QualcommAtherosQCA6174SDIO,
	{vendor: "0x17cb", device: "0x1103"}: QualcommWCN6855,
	{vendor: "0x8086", device: "0x08b1"}: Intel7260,
	{vendor: "0x8086", device: "0x08b2"}: Intel7260,
	{vendor: "0x8086", device: "0x095a"}: Intel7265,
	{vendor: "0x8086", device: "0x095b"}: Intel7265,
	// Note that Intel 9000 is also Intel 9560 aka Jefferson Peak 2
	{vendor: "0x8086", device: "0x9df0"}: Intel9000,
	{vendor: "0x8086", device: "0x31dc"}: Intel9000,
	{vendor: "0x8086", device: "0x2526"}: Intel9260,
	{vendor: "0x8086", device: "0x2723"}: Intel22260,
	// For integrated wifi chips, use device_id and subsystem_id together
	// as an identifier.
	// 0x02f0 is for Quasar on CML; 0x4070, 0x0074, 0x6074 are for HrP2.
	{vendor: "0x8086", device: "0x02f0", subsystem: "0x0034"}: Intel9000,
	{vendor: "0x8086", device: "0x02f0", subsystem: "0x4070"}: Intel22560,
	{vendor: "0x8086", device: "0x02f0", subsystem: "0x0074"}: Intel22560,
	{vendor: "0x8086", device: "0x02f0", subsystem: "0x6074"}: Intel22560,
	{vendor: "0x8086", device: "0x4df0", subsystem: "0x0070"}: Intel22560,
	{vendor: "0x8086", device: "0x4df0", subsystem: "0x4070"}: Intel22560,
	{vendor: "0x8086", device: "0x4df0", subsystem: "0x0074"}: Intel22560,
	{vendor: "0x8086", device: "0x4df0", subsystem: "0x6074"}: Intel22560,
	{vendor: "0x8086", device: "0xa0f0", subsystem: "0x4070"}: Intel22560,
	{vendor: "0x8086", device: "0xa0f0", subsystem: "0x0074"}: Intel22560,
	{vendor: "0x8086", device: "0xa0f0", subsystem: "0x6074"}: Intel22560,
	{vendor: "0x8086", device: "0x51f0", subsystem: "0x0090"}: IntelAX211,
	{vendor: "0x8086", device: "0x51f0", subsystem: "0x0094"}: IntelAX211,
	{vendor: "0x8086", device: "0x54f0", subsystem: "0x0090"}: IntelAX211,
	{vendor: "0x8086", device: "0x54f0", subsystem: "0x0094"}: IntelAX211,
	{vendor: "0x02d0", device: "0x4354"}:                      BroadcomBCM4354SDIO,
	{vendor: "0x14e4", device: "0x43ec"}:                      BroadcomBCM4356PCIE,
	{vendor: "0x14e4", device: "0x440d"}:                      BroadcomBCM4371PCIE,
	{vendor: "0x10ec", device: "0xc822"}:                      Realtek8822CPCIE,
	{vendor: "0x10ec", device: "0x8852"}:                      Realtek8852APCIE,
	{vendor: "0x14c3", device: "0x7961"}:                      MediaTekMT7921PCIE,
	{vendor: "0x037a", device: "0x7901"}:                      MediaTekMT7921SDIO,
	{compatible: "qcom,wcn3990-wifi"}:                         QualcommWCN3990,
	{compatible: "qcom,wcn6750-wifi"}:                         QualcommWCN6750,
}

// DevInfo contains the information of the WLAN device.
type DevInfo struct {
	// vendor is the vendor ID seen in /sys/class/net/<interface>/vendor .
	vendor string
	// device is the product ID seen in /sys/class/net/<interface>/device .
	device string
	// compatible is the compatible property.
	// See https://www.kernel.org/doc/Documentation/devicetree/usage-model.txt .
	compatible string
	// subsystem is the RF chip's ID. This addition of this property is necessary for
	// device disambiguation (b/129489799).
	subsystem string
	// The device name.
	Name string
}

// List of WLAN devices that don't support MU-MIMO.
var denyListMUMIMO = []string{
	Marvell88w8897SDIO,  // Tested a DUT.
	Intel7260,           // (WP2) according to datasheet.
	Intel7265,           // (StP2) tested a DUT.
	BroadcomBCM4354SDIO, // Tested a DUT.
	BroadcomBCM4356PCIE, // According to datasheet.
}

var compatibleRE = regexp.MustCompile("^OF_COMPATIBLE_[0-9]")

// DeviceInfo returns a public struct (DevInfo) that has the WLAN device information.
func DeviceInfo(ctx context.Context, netIf string) (*DevInfo, error) {
	devicePath := filepath.Join("/sys/class/net", netIf, "device")

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

	// Support for (qcom,wcn3990-wifi) and (qcom,wcn6750-wifi) chip.
	for _, line := range strings.Split(uevent, "\n") {
		if kv := compatibleRE.FindStringSubmatch(line); kv != nil {
			if wifiSnoc := strings.Split(line, "="); wifiSnoc != nil {
				if d, ok := lookupWLANDev[DevInfo{compatible: wifiSnoc[1]}]; ok {
					// Found the matching device.
					return &DevInfo{Name: d}, nil
				}
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

	if d, ok := lookupWLANDev[DevInfo{vendor: vendorID, device: productID, subsystem: subsystemID}]; ok {
		return &DevInfo{vendor: vendorID, device: productID, subsystem: subsystemID, Name: d}, nil
	}

	if d, ok := lookupWLANDev[DevInfo{vendor: vendorID, device: productID}]; ok {
		return &DevInfo{vendor: vendorID, device: productID, subsystem: subsystemID, Name: d}, nil
	}

	return nil, errors.Errorf("unknown %s device with vendorID=%s, productID=%s, subsystemID=%s",
		netIf, vendorID, productID, subsystemID)
}

// LogBandwidthSupport logs info about the device bandwidth support.
// For now, it only works for Intel devices.
func LogBandwidthSupport(ctx context.Context, dev *DevInfo) {
	if dev.vendor != intelVendorNum {
		return
	}
	if len(dev.subsystem) < 4 {
		return
	}
	if dev.subsystem[3] == support160MHz {
		testing.ContextLog(ctx, "Bandwidth Support: Supports 160 MHz Bandwidth")
	} else if dev.subsystem[3] == supportOnly80MHz {
		testing.ContextLog(ctx, "Bandwidth Support: Supports only 80 MHz Bandwidth")
	} else {
		testing.ContextLog(ctx, "Bandwidth Support: Doesn't support (80 MHz , 160 MHz) Bandwidth")
	}
}

// SupportMUMIMO return true if the WLAN device support MU-MIMO.
func (dev *DevInfo) SupportMUMIMO() bool {
	// Checking if the tested WLAN device does not support MU-MIMO.
	for _, name := range denyListMUMIMO {
		if name == dev.Name {
			return false
		}
	}
	return true
}
