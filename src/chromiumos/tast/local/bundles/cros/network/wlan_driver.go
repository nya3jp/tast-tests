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
			"kirtika@chromium.org", // Connectivity team
			"oka@chromium.org",     // Tast port author
		},
		Attr: []string{"informational"},
	})
}

// WLAN device names
const (
	marvell88w8797SDIO         = "Marvell 88W8797 SDIO"
	marvell88w8887SDIO         = "Marvell 88W8887 SDIO"
	marvell88w8897SDIO         = "Marvell 88W8897 SDIO"
	marvell88w8897Pcie         = "Marvell 88W8897 PCIE"
	marvell88w8997Pcie         = "Marvell 88W8997 PCIE"
	atherosAr9280              = "Atheros AR9280"
	atherosAr9382              = "Atheros AR9382"
	atherosAr9462              = "Atheros AR9462"
	qualcommAtherosQca6174     = "Qualcomm Atheros QCA6174"
	qualcommAtherosQca6174SDIO = "Qualcomm Atheros QCA6174 SDIO"
	qualcommWcn3990            = "Qualcomm WCN3990"
	intel7260                  = "Intel 7260"
	intel7265                  = "Intel 7265"
	intel9000                  = "Intel 9000"
	intel9260                  = "Intel 9260"
	intel22260                 = "Intel 22260"
	broadcomBcm4354SDIO        = "Broadcom BCM4354 SDIO"
	broadcomBcm4356Pcie        = "Broadcom BCM4356 PCIE"
	broadcomBcm4371Pcie        = "Broadcom BCM4371 PCIE"
)

type wlanDeviceInfo struct {
	// The vendor ID seen in /sys/class/net/<interface>/vendor .
	vendor string
	// The product ID seen in /sys/class/net/<interface>/device .
	device string
	// The compatible property.
	// See https://www.kernel.org/doc/Documentation/devicetree/usage-model.txt .
	compatible string
}

//

var wlanDeviceLookup = map[wlanDeviceInfo]string{
	{vendor: "0x02df", device: "0x9129"}: marvell88w8797SDIO,
	{vendor: "0x02df", device: "0x912d"}: marvell88w8897SDIO,
	{vendor: "0x02df", device: "0x9135"}: marvell88w8887SDIO,
	{vendor: "0x11ab", device: "0x2b38"}: marvell88w8897Pcie,
	{vendor: "0x1b4b", device: "0x2b42"}: marvell88w8997Pcie,
	{vendor: "0x168c", device: "0x002a"}: atherosAr9280,
	{vendor: "0x168c", device: "0x0030"}: atherosAr9382,
	{vendor: "0x168c", device: "0x0034"}: atherosAr9462,
	{vendor: "0x168c", device: "0x003e"}: qualcommAtherosQca6174,
	{vendor: "0x105b", device: "0xe09d"}: qualcommAtherosQca6174,
	{vendor: "0x0271", device: "0x050a"}: qualcommAtherosQca6174SDIO,
	{vendor: "0x8086", device: "0x08b1"}: intel7260,
	{vendor: "0x8086", device: "0x08b2"}: intel7260,
	{vendor: "0x8086", device: "0x095a"}: intel7265,
	{vendor: "0x8086", device: "0x095b"}: intel7265,
	{vendor: "0x8086", device: "0x9df0"}: intel9000,
	{vendor: "0x8086", device: "0x31dc"}: intel9000,
	{vendor: "0x8086", device: "0x2526"}: intel9260,
	{vendor: "0x8086", device: "0x2723"}: intel22260,
	{vendor: "0x02d0", device: "0x4354"}: broadcomBcm4354SDIO,
	{vendor: "0x14e4", device: "0x43ec"}: broadcomBcm4356Pcie,
	{vendor: "0x14e4", device: "0x440d"}: broadcomBcm4371Pcie,
	{compatible: "qcom,wcn3990-wifi"}:    qualcommWcn3990,
}

var expectedWLANDriver = map[string]map[string]string{
	atherosAr9280: {
		"3.4":  "wireless/ath/ath9k/ath9k.ko",
		"3.8":  "wireless-3.4/ath/ath9k/ath9k.ko",
		"4.14": "wireless/ath/ath9k/ath9k.ko",
		"4.19": "wireless/ath/ath9k/ath9k.ko",
	},
	atherosAr9382: {
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
	atherosAr9462: {
		"3.4":  "wireless/ath/ath9k_btcoex/ath9k_btcoex.ko",
		"3.8":  "wireless-3.4/ath/ath9k_btcoex/ath9k_btcoex.ko",
		"4.14": "wireless/ath/ath9k/ath9k.ko",
		"4.19": "wireless/ath/ath9k/ath9k.ko",
	},
	qualcommAtherosQca6174: {
		"4.4":  "wireless/ar10k/ath/ath10k/ath10k_pci.ko",
		"4.14": "wireless/ath/ath10k/ath10k_pci.ko",
		"4.19": "wireless/ath/ath10k/ath10k_pci.ko",
	},
	qualcommAtherosQca6174SDIO: {
		"4.19": "wireless/ath/ath10k/ath10k_sdio.ko",
	},
	qualcommWcn3990: {
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
	marvell88w8897Pcie: {
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
	broadcomBcm4354SDIO: {
		"3.8":  "wireless/brcm80211/brcmfmac/brcmfmac.ko",
		"3.14": "wireless-3.8/brcm80211/brcmfmac/brcmfmac.ko",
		"4.14": "wireless/broadcom/brcm80211/brcmfmac/brcmfmac.ko",
		"4.19": "wireless/broadcom/brcm80211/brcmfmac/brcmfmac.ko",
	},
	broadcomBcm4356Pcie: {
		"3.10": "wireless-3.8/brcm80211/brcmfmac/brcmfmac.ko",
		"4.14": "wireless/broadcom/brcm80211/brcmfmac/brcmfmac.ko",
		"4.19": "wireless/broadcom/brcm80211/brcmfmac/brcmfmac.ko",
	},
	marvell88w8997Pcie: {
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

	if want, got := filepath.Base(expectedPath), moduleName+".ko"; want != got {
		s.Error("Module name is %s, want %s", got, want)
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

// getWLANDeviceName returns the device name of the given wireless network interface, or returns an error on failure.
func getWLANDeviceName(ctx context.Context, netIf string) (string, error) {
	devicePath := filepath.Join("/sys/class/net", netIf, "device")

	readInfo := func(x string) (string, error) {
		bs, err := ioutil.ReadFile(filepath.Join(devicePath, x))
		return strings.TrimSpace(string(bs)), err
	}

	vendorID, err := readInfo("vendor")
	if err != nil {
		return "", errors.Wrapf(err, "get device %s: failed to get vendor ID", netIf)
	}
	productID, err := readInfo("device")
	if err != nil {
		return "", errors.Wrapf(err, "get device %s: failed to get product ID", netIf)
	}
	uevent, err := readInfo("uevent")
	if err != nil {
		return "", errors.Wrapf(err, "get device %s: failed to get uevent", netIf)
	}

	devices := []wlanDeviceInfo{{vendor: vendorID, device: productID}}

	for _, line := range strings.Split(uevent, "\n") {
		kv := strings.Split(line, "=")
		if regexp.MustCompile("^OF_COMPATIBLE_[0-9]+$").MatchString(kv[0]) {
			devices = append(devices, wlanDeviceInfo{
				compatible: kv[1],
			})
		}
	}

	for _, info := range devices {
		if d, ok := wlanDeviceLookup[info]; ok {
			return d, nil
		}
	}
	return "", errors.Errorf("get device %s: device unknown", netIf)
}
