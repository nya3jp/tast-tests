// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/local/bundles/cros/wifi/wlan"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Driver,
		Desc: "Ensure wireless devices have the expected associated kernel driver",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		// Run on both Tast CQ and suite:wifi_matfunc.
		Attr:         []string{"group:mainline", "group:wificell", "wificell_func"},
		SoftwareDeps: []string{"wifi", "shill-wifi", "no_kernel_upstream"},
	})
}

var expectedWLANDriver = map[string]map[string]string{
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
		"5.4":  "wireless/iwl7000/iwlwifi/iwlwifi.ko",
		"5.10": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
	},
	wlan.Intel9000: {
		"4.14": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
		"4.19": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
	},
	wlan.Intel9260: {
		"4.4":  "wireless/iwl7000/iwlwifi/iwlwifi.ko",
		"4.14": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
		"4.19": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
		"5.4":  "wireless/iwl7000/iwlwifi/iwlwifi.ko",
	},
	wlan.Intel22260: {
		"4.4":  "wireless/iwl7000/iwlwifi/iwlwifi.ko",
		"4.14": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
		"4.19": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
		"5.4":  "wireless/iwl7000/iwlwifi/iwlwifi.ko",
		"5.10": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
	},
	wlan.Intel22560: {
		"4.19": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
		"5.4":  "wireless/iwl7000/iwlwifi/iwlwifi.ko",
		"5.10": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
	},
	wlan.IntelAX211: {
		"5.10": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
	},
	wlan.QualcommAtherosQCA6174: {
		"4.4":  "wireless/ar10k/ath/ath10k/ath10k_pci.ko",
		"4.14": "wireless/ath/ath10k/ath10k_pci.ko",
		"4.19": "wireless/ath/ath10k/ath10k_pci.ko",
		"5.4":  "wireless/ath/ath10k/ath10k_pci.ko",
		"5.10": "wireless/ath/ath10k/ath10k_pci.ko",
	},
	wlan.QualcommAtherosQCA6174SDIO: {
		"4.19": "wireless/ath/ath10k/ath10k_sdio.ko",
		"5.10": "wireless/ath/ath10k/ath10k_sdio.ko",
	},
	wlan.QualcommWCN3990: {
		"4.14": "wireless/ath/ath10k/ath10k_snoc.ko",
		"4.19": "wireless/ath/ath10k/ath10k_snoc.ko",
		"5.4":  "wireless/ath/ath10k/ath10k_snoc.ko",
		"5.10": "wireless/ath/ath10k/ath10k_snoc.ko",
	},
	wlan.Marvell88w8897SDIO: {
		"3.8":  "wireless/mwifiex/mwifiex_sdio.ko",
		"3.10": "wireless-3.8/mwifiex/mwifiex_sdio.ko",
		"3.14": "wireless-3.8/mwifiex/mwifiex_sdio.ko",
		"3.18": "wireless/mwifiex/mwifiex_sdio.ko",
		"4.14": "wireless/marvell/mwifiex/mwifiex_sdio.ko",
		"4.19": "wireless/marvell/mwifiex/mwifiex_sdio.ko",
		"5.4":  "wireless/marvell/mwifiex/mwifiex_sdio.ko",
		"5.10": "wireless/marvell/mwifiex/mwifiex_sdio.ko",
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
		"5.4":  "wireless/marvell/mwifiex/mwifiex_pcie.ko",
		"5.10": "wireless/marvell/mwifiex/mwifiex_pcie.ko",
	},
	wlan.Realtek8822CPCIE: {
		"4.14": "wireless/realtek/rtw88/rtw88_8822ce.ko",
		"5.4":  "wireless/realtek/rtw88/rtw88_8822ce.ko",
		"5.10": "wireless/realtek/rtw88/rtw88_8822ce.ko",
	},
	wlan.Realtek8852APCIE: {
		"5.4":  "wireless/realtek/rtw89/rtw89_pci.ko",
		"5.10": "wireless/realtek/rtw89/rtw89_pci.ko",
	},
	wlan.MediaTekMT7921PCIE: {
		"5.4": "wireless/mediatek/mt76/mt7921/mt7921e.ko",
	},
}

func Driver(ctx context.Context, s *testing.State) {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}

	netIf, err := shill.WifiInterface(ctx, manager, 5*time.Second)
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
