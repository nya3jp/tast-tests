// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/local/bundles/cros/network/wlan"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/sysutil"
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
		// TODO(crbug.com/1007252): Remove informational after fixing flakiness.
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"wifi"},

		// TODO(crbug.com/984433): Consider skipping nyan_kitty. It has been skipped in the original test as it's unresolvably flaky (crrev.com/c/944502), exhibiting very similar symptoms to crbug.com/693724, b/65858242, b/36264732.
	})
}

var expectedWLANDriver = map[string]map[string]string{
	wlan.AtherosAR9280: {
		"3.4":  "wireless/ath/ath9k/ath9k.ko",
		"3.8":  "wireless-3.4/ath/ath9k/ath9k.ko",
		"4.14": "wireless/ath/ath9k/ath9k.ko",
		"4.19": "wireless/ath/ath9k/ath9k.ko",
	},
	wlan.AtherosAR9382: {
		"3.4":  "wireless/ath/ath9k/ath9k.ko",
		"3.8":  "wireless-3.4/ath/ath9k/ath9k.ko",
		"4.14": "wireless/ath/ath9k/ath9k.ko",
		"4.19": "wireless/ath/ath9k/ath9k.ko",
	},
	wlan.Intel7260: {
		"3.8":  "wireless/iwl7000/iwlwifi/iwlwifi.ko",
		"3.14": "wireless-3.8/iwl7000/iwlwifi/iwlwifi.ko",
		"4.4":  "wireless/iwl7000/iwlwifi/iwlwifi.ko",
		"4.14": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
		"4.19": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
	},
	wlan.Intel7265: {
		"3.8":  "wireless/iwl7000/iwlwifi/iwlwifi.ko",
		"3.14": "wireless-3.8/iwl7000/iwlwifi/iwlwifi.ko",
		"3.18": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
		"4.4":  "wireless/iwl7000/iwlwifi/iwlwifi.ko",
		"4.14": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
		"4.19": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
	},
	wlan.Intel9000: {
		"4.14": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
		"4.19": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
	},
	wlan.Intel9260: {
		"4.4":  "wireless/iwl7000/iwlwifi/iwlwifi.ko",
		"4.14": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
		"4.19": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
	},
	wlan.Intel22260: {
		"4.4":  "wireless/iwl7000/iwlwifi/iwlwifi.ko",
		"4.14": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
		"4.19": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
	},
	wlan.Intel22560: {
		"4.19": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
	},
	wlan.AtherosAR9462: {
		"3.4":  "wireless/ath/ath9k_btcoex/ath9k_btcoex.ko",
		"3.8":  "wireless-3.4/ath/ath9k_btcoex/ath9k_btcoex.ko",
		"4.14": "wireless/ath/ath9k/ath9k.ko",
		"4.19": "wireless/ath/ath9k/ath9k.ko",
	},
	wlan.QualcommAtherosQCA6174: {
		"4.4":  "wireless/ar10k/ath/ath10k/ath10k_pci.ko",
		"4.14": "wireless/ath/ath10k/ath10k_pci.ko",
		"4.19": "wireless/ath/ath10k/ath10k_pci.ko",
	},
	wlan.QualcommAtherosQCA6174SDIO: {
		"4.19": "wireless/ath/ath10k/ath10k_sdio.ko",
	},
	wlan.QualcommWCN3990: {
		"4.14": "wireless/ath/ath10k/ath10k_snoc.ko",
		"4.19": "wireless/ath/ath10k/ath10k_snoc.ko",
	},
	wlan.Marvell88w8797SDIO: {
		"3.4":  "wireless/mwifiex/mwifiex_sdio.ko",
		"3.8":  "wireless-3.4/mwifiex/mwifiex_sdio.ko",
		"4.14": "wireless/marvell/mwifiex/mwifiex_sdio.ko",
		"4.19": "wireless/marvell/mwifiex/mwifiex_sdio.ko",
	},
	wlan.Marvell88w8887SDIO: {
		"3.14": "wireless-3.8/mwifiex/mwifiex_sdio.ko",
		"4.14": "wireless/marvell/mwifiex/mwifiex_sdio.ko",
		"4.19": "wireless/marvell/mwifiex/mwifiex_sdio.ko",
	},
	wlan.Marvell88w8897PCIE: {
		"3.8":  "wireless/mwifiex/mwifiex_pcie.ko",
		"3.10": "wireless-3.8/mwifiex/mwifiex_pcie.ko",
		"4.14": "wireless/marvell/mwifiex/mwifiex_pcie.ko",
		"4.19": "wireless/marvell/mwifiex/mwifiex_pcie.ko",
	},
	wlan.Marvell88w8897SDIO: {
		"3.8":  "wireless/mwifiex/mwifiex_sdio.ko",
		"3.10": "wireless-3.8/mwifiex/mwifiex_sdio.ko",
		"3.14": "wireless-3.8/mwifiex/mwifiex_sdio.ko",
		"3.18": "wireless/mwifiex/mwifiex_sdio.ko",
		"4.14": "wireless/marvell/mwifiex/mwifiex_sdio.ko",
		"4.19": "wireless/marvell/mwifiex/mwifiex_sdio.ko",
	},
	wlan.BroadcomBCM4354SDIO: {
		"3.8":  "wireless/brcm80211/brcmfmac/brcmfmac.ko",
		"3.14": "wireless-3.8/brcm80211/brcmfmac/brcmfmac.ko",
		"4.14": "wireless/broadcom/brcm80211/brcmfmac/brcmfmac.ko",
		"4.19": "wireless/broadcom/brcm80211/brcmfmac/brcmfmac.ko",
	},
	wlan.BroadcomBCM4356PCIE: {
		"3.10": "wireless-3.8/brcm80211/brcmfmac/brcmfmac.ko",
		"4.14": "wireless/broadcom/brcm80211/brcmfmac/brcmfmac.ko",
		"4.19": "wireless/broadcom/brcm80211/brcmfmac/brcmfmac.ko",
	},
	wlan.Marvell88w8997PCIE: {
		"4.4":  "wireless/marvell/mwifiex/mwifiex_pcie.ko",
		"4.14": "wireless/marvell/mwifiex/mwifiex_pcie.ko",
		"4.19": "wireless/marvell/mwifiex/mwifiex_pcie.ko",
	},
	wlan.Realtek8822CPCIE: {
		"4.14": "wireless/realtek/rtw88/rtwpci.ko",
	},
}

func WLANDriver(ctx context.Context, s *testing.State) {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}

	netIf, err := shill.GetWifiInterface(ctx, manager, 5*time.Second)
	if err != nil {
		s.Fatal(err, "Could not get a WiFi interface: ", err)
	}

	// TODO(oka): Original test skips if "wifi" is not initialized (USE="-wifi").
	// Consider if we should do it.
	// https://chromium-review.googlesource.com/c/chromiumos/third_party/autotest/+/890121
	devInfo, err := wlan.DeviceInfo(ctx, netIf)
	if err != nil {
		s.Fatal("Failed to get device name: ", err)
	}

	// If the device is Intel, check if it supports
	// 160 MHz / 80 MHz wide channels and log this information.
	wlan.LogBandwidthSupport(ctx, devInfo)

	if _, ok := expectedWLANDriver[devInfo.Name]; !ok {
		s.Fatal("Unexpected device ", devInfo.Name)
	}

	u, err := sysutil.Uname()
	if err != nil {
		s.Fatal("Failed to get uname: ", err)
	}
	baseRevision := strings.Join(strings.Split(u.Release, ".")[:2], ".")

	expectedPath, ok := expectedWLANDriver[devInfo.Name][baseRevision]
	if !ok {
		s.Fatalf("Unexpected base revision %v for device %v", baseRevision, devInfo.Name)
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
