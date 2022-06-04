// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/wlan"
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

var expectedWLANDriver = map[wlan.DeviceID]map[string]string{
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
		"5.10": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
		"5.15": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
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
		"5.15": "wireless/iwl7000/iwlwifi/iwlwifi.ko",
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
		"5.15": "wireless/ath/ath10k/ath10k_snoc.ko",
	},
	wlan.QualcommWCN6750: {
		"5.15": "wireless/ath/ath11k/ath11k_ahb.ko",
	},
	wlan.QualcommWCN6855: {
		"5.10": "wireless/ath/ath11k/ath11k_pci.ko",
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
		"5.15": "wireless/realtek/rtw88/rtw88_8822ce.ko",
	},
	wlan.Realtek8852APCIE: {
		"5.4":  "wireless/realtek/rtw89/rtw89_pci.ko",
		"5.10": "wireless/realtek/rtw89/rtw89_pci.ko",
	},
	wlan.MediaTekMT7921PCIE: {
		"5.4":  "wireless/mediatek/mt76/mt7921/mt7921e.ko",
		"5.10": "wireless/mediatek/mt76/mt7921/mt7921e.ko",
		"5.15": "wireless/mediatek/mt76/mt7921/mt7921e.ko",
	},
	wlan.MediaTekMT7921SDIO: {
		"5.10": "wireless/mediatek/mt76/mt7921/mt7921s.ko",
	},
}

func Driver(ctx context.Context, s *testing.State) {
	const (
		intelVendorNum   = "0x8086"
		support160MHz    = '0'
		supportOnly80MHz = '2'
	)

	logBandwidthSupport := func(ctx context.Context, dev *wlan.DevInfo) {
		if dev.Vendor != intelVendorNum {
			return
		}
		if len(dev.Subsystem) < 4 {
			return
		}
		if dev.Subsystem[3] == support160MHz {
			testing.ContextLog(ctx, "Bandwidth Support: Supports 160 MHz Bandwidth")
		} else if dev.Subsystem[3] == supportOnly80MHz {
			testing.ContextLog(ctx, "Bandwidth Support: Supports only 80 MHz Bandwidth")
		} else {
			testing.ContextLog(ctx, "Bandwidth Support: Doesn't support (80 MHz , 160 MHz) Bandwidth")
		}
	}

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
	devInfo, err := wlan.DeviceInfo()
	if err != nil {
		s.Fatal("Failed to get device name: ", err)
	}

	// If the device is Intel, check if it supports
	// 160 MHz / 80 MHz wide channels and log this information.
	logBandwidthSupport(ctx, devInfo)

	if _, ok := expectedWLANDriver[devInfo.ID]; !ok {
		s.Fatal("Unexpected device ", devInfo.Name)
	}

	u, err := sysutil.Uname()
	if err != nil {
		s.Fatal("Failed to get uname: ", err)
	}
	baseRevision := strings.Join(strings.Split(u.Release, ".")[:2], ".")

	expectedPath, ok := expectedWLANDriver[devInfo.ID][baseRevision]
	if !ok {
		s.Fatalf("Unexpected base revision %v for device %v", baseRevision, devInfo.Name)
	}

	netDriversRoot := filepath.Join("/lib/modules", u.Release, "kernel/drivers/net")
	expectedModulePath := filepath.Join(netDriversRoot, expectedPath)

	if info, err := os.Stat(filepath.Dir(expectedModulePath)); err != nil {
		s.Error("Failed to stat module dir: ", err)
	} else if !info.IsDir() {
		s.Errorf("%v is not a directory", filepath.Dir(expectedModulePath))
	}

	if match, _ := filepath.Glob(expectedModulePath + "*"); match == nil {
		s.Errorf("Failed to locate module matching %v*", expectedModulePath)
	}

	moduleDir := filepath.Join("/sys/class/net", netIf, "device/driver/")
	dirs, err := ioutil.ReadDir(moduleDir)
	if err != nil {
		s.Fatal("Failed to list module path: ", err)
	}
	var path string
	for _, dir := range dirs {
		// Most of the devices link module under device/driver/module.
		if dir.Name() == "module" {
			modulePath := filepath.Join(moduleDir, "module")
			path, err = os.Readlink(modulePath)
			if err != nil {
				s.Fatal("Failed to readlink module path: ", err)
			}
			break
		}
		// Some SDIO devices may keep module link in device/driver/mmc?:????:?/driver.
		if match, _ := filepath.Match("mmc*", dir.Name()); match {
			modulePath := filepath.Join(moduleDir, dir.Name(), "driver")
			path, err = os.Readlink(modulePath)
			if err != nil {
				s.Fatal("Failed to readlink module path: ", err)
			}
			break
		}
	}
	if path == "" {
		s.Fatal("Failed to locate module path: ", err)
	}
	moduleName := filepath.Base(path)

	if got, want := moduleName+".ko", filepath.Base(expectedPath); got != want {
		s.Errorf("Module name is %s, want %s", got, want)
	}
}
