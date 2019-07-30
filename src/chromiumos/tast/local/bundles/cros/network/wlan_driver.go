// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: WLANDriver,
		Desc: "Ensure wireless devices have the expected associated kernel driver",
		Contacts: []string{
			"briannorris@chromium.org",        // Connectivity team
			"chromeos-kernel-wifi@google.com", // Connectivity team
			"oka@chromium.org",                // Tast port author
		},
		Attr: []string{"informational"},
		// TODO(crbug.com/982620): Add SoftwareDeps for wifi.

		// TODO(crbug.com/984433): Consider skipping nyan_kitty. It has been skipped in the original test as it's unresolvably flaky (crrev.com/c/944502), exhibiting very similar symptoms to crbug.com/693724, b/65858242, b/36264732.
	})
}

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
)

type wlanDeviceInfo struct {
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
}

var wlanDeviceLookup = map[wlanDeviceInfo]string{
	{vendor: "0x02df", device: "0x9129"}: marvell88w8797SDIO,
	{vendor: "0x02df", device: "0x912d"}: marvell88w8897SDIO,
	{vendor: "0x02df", device: "0x9135"}: marvell88w8887SDIO,
	{vendor: "0x11ab", device: "0x2b38"}: marvell88w8897PCIE,
	{vendor: "0x1b4b", device: "0x2b42"}: marvell88w8997PCIE,
	{vendor: "0x168c", device: "0x002a"}: atherosAR9280,
	{vendor: "0x168c", device: "0x0030"}: atherosAR9382,
	{vendor: "0x168c", device: "0x0034"}: atherosAR9462,
	{vendor: "0x168c", device: "0x003e"}: qualcommAtherosQCA6174,
	{vendor: "0x105b", device: "0xe09d"}: qualcommAtherosQCA6174,
	{vendor: "0x0271", device: "0x050a"}: qualcommAtherosQCA6174SDIO,
	{vendor: "0x8086", device: "0x08b1"}: intel7260,
	{vendor: "0x8086", device: "0x08b2"}: intel7260,
	{vendor: "0x8086", device: "0x095a"}: intel7265,
	{vendor: "0x8086", device: "0x095b"}: intel7265,
	// Note that Intel 9000 is also Intel 9560 aka Jefferson Peak 2
	{vendor: "0x8086", device: "0x9df0"}: intel9000,
	{vendor: "0x8086", device: "0x31dc"}: intel9000,
	{vendor: "0x8086", device: "0x2526"}: intel9260,
	{vendor: "0x8086", device: "0x2723"}: intel22260,
	// For integrated wifi chips, use device_id and subsystem_id together
	// as an identifier.
	// 0x02f0 is for Quasar on CML, 0x0074 is for HrP2
	{vendor: "0x8086", device: "0x02f0", subsystem: "0x0034"}: intel9000,
	{vendor: "0x8086", device: "0x02f0", subsystem: "0x0074"}: intel22560,
	{vendor: "0x02d0", device: "0x4354"}:                      broadcomBCM4354SDIO,
	{vendor: "0x14e4", device: "0x43ec"}:                      broadcomBCM4356PCIE,
	{vendor: "0x14e4", device: "0x440d"}:                      broadcomBCM4371PCIE,
	{compatible: "qcom,wcn3990-wifi"}:                         qualcommWCN3990,
}

var expectedWLANDriver = map[string]map[string]string{
	atherosAR9280: {
		"3.4":  "wireless/ath/ath9k/ath9k.ko",
		"3.8":  "wireless-3.4/ath/ath9k/ath9k.ko",
		"4.14": "wireless/ath/ath9k/ath9k.ko",
		"4.19": "wireless/ath/ath9k/ath9k.ko",
	},
	atherosAR9382: {
		"3.4":  "wireless/ath/ath9k/ath9k.ko",
		"3.8":  "wireless-3.4/ath/ath9k/ath9k.ko",
		"4.14": "wireless/ath/ath9k/ath9k.ko",
		"4.19": "wireless/ath/ath9k/ath9k.ko",
	},
	intel7260: {
		"3.8":  "wireless/iwl7000/iwlwifi/iwlwifi.ko",
		"3.14": "wireless-3.8/iwl7000/iwlwifi/iwlwifi.ko",
		"4.4":  "wireless/iwl7000/iwlwifi/iwlwifi.ko",
		"4.14": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
		"4.19": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
	},
	intel7265: {
		"3.8":  "wireless/iwl7000/iwlwifi/iwlwifi.ko",
		"3.14": "wireless-3.8/iwl7000/iwlwifi/iwlwifi.ko",
		"3.18": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
		"4.4":  "wireless/iwl7000/iwlwifi/iwlwifi.ko",
		"4.14": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
		"4.19": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
	},
	intel9000: {
		"4.14": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
		"4.19": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
	},
	intel9260: {
		"4.4":  "wireless/iwl7000/iwlwifi/iwlwifi.ko",
		"4.14": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
		"4.19": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
	},
	intel22260: {
		"4.4":  "wireless/iwl7000/iwlwifi/iwlwifi.ko",
		"4.14": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
		"4.19": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
	},
	intel22560: {
		"4.19": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
	},
	atherosAR9462: {
		"3.4":  "wireless/ath/ath9k_btcoex/ath9k_btcoex.ko",
		"3.8":  "wireless-3.4/ath/ath9k_btcoex/ath9k_btcoex.ko",
		"4.14": "wireless/ath/ath9k/ath9k.ko",
		"4.19": "wireless/ath/ath9k/ath9k.ko",
	},
	qualcommAtherosQCA6174: {
		"4.4":  "wireless/ar10k/ath/ath10k/ath10k_pci.ko",
		"4.14": "wireless/ath/ath10k/ath10k_pci.ko",
		"4.19": "wireless/ath/ath10k/ath10k_pci.ko",
	},
	qualcommAtherosQCA6174SDIO: {
		"4.19": "wireless/ath/ath10k/ath10k_sdio.ko",
	},
	qualcommWCN3990: {
		"4.14": "wireless/ath/ath10k/ath10k_snoc.ko",
		"4.19": "wireless/ath/ath10k/ath10k_snoc.ko",
	},
	marvell88w8797SDIO: {
		"3.4":  "wireless/mwifiex/mwifiex_sdio.ko",
		"3.8":  "wireless-3.4/mwifiex/mwifiex_sdio.ko",
		"4.14": "wireless/marvell/mwifiex/mwifiex_sdio.ko",
		"4.19": "wireless/marvell/mwifiex/mwifiex_sdio.ko",
	},
	marvell88w8887SDIO: {
		"3.14": "wireless-3.8/mwifiex/mwifiex_sdio.ko",
		"4.14": "wireless/marvell/mwifiex/mwifiex_sdio.ko",
		"4.19": "wireless/marvell/mwifiex/mwifiex_sdio.ko",
	},
	marvell88w8897PCIE: {
		"3.8":  "wireless/mwifiex/mwifiex_pcie.ko",
		"3.10": "wireless-3.8/mwifiex/mwifiex_pcie.ko",
		"4.14": "wireless/marvell/mwifiex/mwifiex_pcie.ko",
		"4.19": "wireless/marvell/mwifiex/mwifiex_pcie.ko",
	},
	marvell88w8897SDIO: {
		"3.8":  "wireless/mwifiex/mwifiex_sdio.ko",
		"3.10": "wireless-3.8/mwifiex/mwifiex_sdio.ko",
		"3.14": "wireless-3.8/mwifiex/mwifiex_sdio.ko",
		"3.18": "wireless/mwifiex/mwifiex_sdio.ko",
		"4.14": "wireless/marvell/mwifiex/mwifiex_sdio.ko",
		"4.19": "wireless/marvell/mwifiex/mwifiex_sdio.ko",
	},
	broadcomBCM4354SDIO: {
		"3.8":  "wireless/brcm80211/brcmfmac/brcmfmac.ko",
		"3.14": "wireless-3.8/brcm80211/brcmfmac/brcmfmac.ko",
		"4.14": "wireless/broadcom/brcm80211/brcmfmac/brcmfmac.ko",
		"4.19": "wireless/broadcom/brcm80211/brcmfmac/brcmfmac.ko",
	},
	broadcomBCM4356PCIE: {
		"3.10": "wireless-3.8/brcm80211/brcmfmac/brcmfmac.ko",
		"4.14": "wireless/broadcom/brcm80211/brcmfmac/brcmfmac.ko",
		"4.19": "wireless/broadcom/brcm80211/brcmfmac/brcmfmac.ko",
	},
	marvell88w8997PCIE: {
		"4.4":  "wireless/marvell/mwifiex/mwifiex_pcie.ko",
		"4.14": "wireless/marvell/mwifiex/mwifiex_pcie.ko",
		"4.19": "wireless/marvell/mwifiex/mwifiex_pcie.ko",
	},
}

func WLANDriver(ctx context.Context, s *testing.State) {
	netIf, err := getWLANInterface(ctx)
	if err != nil {
		s.Fatal("Failed to get network interface name: ", err)
	}
	// TODO(oka): Original test skips if "wifi" is not initialized (USE="-wifi").
	// Consider if we should do it.
	// https://chromium-review.googlesource.com/c/chromiumos/third_party/autotest/+/890121
	deviceName, err := getWLANDeviceName(ctx, netIf)
	if err != nil {
		s.Fatal("Failed to get device name: ", err)
	}
	if _, ok := expectedWLANDriver[deviceName]; !ok {
		s.Fatal("Unexpected device ", deviceName)
	}

	u, err := sysutil.Uname()
	if err != nil {
		s.Fatal("Failed to get uname: ", err)
	}
	baseRevision := strings.Join(strings.Split(u.Release, ".")[:2], ".")

	expectedPath, ok := expectedWLANDriver[deviceName][baseRevision]
	if !ok {
		s.Fatalf("Unexpected base revision %v for device %v", baseRevision, deviceName)
	}

	netDriversRoot := filepath.Join("/lib/modules", u.Release, "kernel/drivers/net")
	expectedModulePath := filepath.Join(netDriversRoot, expectedPath)

	if _, err := os.Stat(expectedModulePath); err != nil {
		if os.IsNotExist(err) {
			s.Error("Module does not exist: ", err)
		} else {
			s.Error("Failed to stat module path: ", err)
		}
	}

	modulePath := filepath.Join("/sys/class/net", netIf, "device/driver/module")
	rel, err := os.Readlink(modulePath)
	if err != nil {
		s.Fatal("Failed to readlink module path: ", err)
	}
	moduleName := filepath.Base(rel)

	if got, want := moduleName+".ko", filepath.Base(expectedPath); got != want {
		s.Errorf("Module name is %s, want %s", got, want)
	}
}

// getWLANInterface returns the wireless device interface name (e.g. wlan0), or returns an error on failure.
// It goes through the interface names in /sys/class/net and returns one which represents a wireless device interface.
func getWLANInterface(ctx context.Context) (string, error) {
	nics, err := ioutil.ReadDir("/sys/class/net")
	if err != nil {
		return "", errors.Wrap(err, "failed to read device info")
	}

	isWLANInterface := func(netIf string) (bool, error) {
		cmd := testexec.CommandContext(ctx, "iw", "dev", netIf, "info")
		if err := cmd.Run(); err == nil {
			return true, nil
		} else if _, ok := err.(*exec.ExitError); ok {
			return false, nil
		} else {
			return false, errors.Wrapf(err, "failed to get the device status for %s", netIf)
		}
	}

	for _, nic := range nics {
		if ok, err := isWLANInterface(nic.Name()); err != nil {
			return "", err
		} else if ok {
			return nic.Name(), nil
		}
	}
	return "", errors.New("found no recognized wireless device")
}

var ofCompatibleRE = regexp.MustCompile("^OF_COMPATIBLE_[0-9]+$")

// getWLANDeviceName returns the device name of the given wireless network interface, or returns an error on failure.
func getWLANDeviceName(ctx context.Context, netIf string) (string, error) {
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
		return "", errors.Wrapf(err, "get device %s: failed to get uevent", netIf)
	}
	for _, line := range strings.Split(uevent, "\n") {
		kv := strings.Split(line, "=")
		if ofCompatibleRE.MatchString(kv[0]) {
			if d, ok := wlanDeviceLookup[wlanDeviceInfo{compatible: kv[1]}]; ok {
				// Found the matching device.
				return d, nil
			}
		}
	}

	vendorID, err := readInfo("vendor")
	if err != nil {
		return "", errors.Wrapf(err, "get device %s: failed to get vendor ID", netIf)
	}
	productID, err := readInfo("device")
	if err != nil {
		return "", errors.Wrapf(err, "get device %s: failed to get product ID", netIf)
	}
	// DUTs that use SDIO as the bus technology may not have subsystem_device at all.
	// If this is the case, just use an ID of empty string instead.
	subsystemID, err := readInfo("subsystem_device")
	if err != nil && !os.IsNotExist(err) {
		return "", errors.Wrap(err, "error reading subsystem_device")
	}

	if d, ok := wlanDeviceLookup[wlanDeviceInfo{vendor: vendorID, device: productID, subsystem: subsystemID}]; ok {
		return d, nil
	} else if d, ok := wlanDeviceLookup[wlanDeviceInfo{vendor: vendorID, device: productID}]; ok {
		return d, nil
	}
	return "", errors.Errorf("get device %s: device unknown", netIf)
}
