// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"

	"chromiumos/tast/common/network/iw"
	localiw "chromiumos/tast/local/network/iw"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SAPCapabilities,
		Desc: "Verifies that DUT has necessary capabilities to run Soft AP",
		Contacts: []string{
			"jck@semihalf.com",                // Test author
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:         []string{"group:wificell_cross_device", "wificell_cross_device_sap", "wificell_cross_device_unstable"},
		SoftwareDeps: []string{"wifi", "shill-wifi"},
		HardwareDeps: hwdep.D(hwdep.WifiSAP()),
	})
}

// SAPCapabilities verifies level of support for Soft AP according to the driver.
func SAPCapabilities(ctx context.Context, s *testing.State) {
	/*
		This test checks that DUT has necessary capabilities to run Soft AP (MVP):
		1- Supports AP mode,
		2- 2.GHz band.
		Also, it detects opportunities for more functionalities:
		3- Concurrent Mode with AP,
		4- Max number of Clients (if information is present).
	*/
	phys, _, err := localiw.NewLocalRunner().ListPhys(ctx)
	if err != nil {
		s.Error("Failed to get capabilities: ", err)
	}

	checkMode := func(phy *iw.Phy, mode iw.IfType) bool {
		for _, m := range phy.Modes {
			if m == string(mode) {
				testing.ContextLogf(ctx, "Device supports %s mode", mode)
				return true
			}
		}
		return false
	}

	checkBands := func(phy *iw.Phy) bool {
		var has24GHz bool
		for _, b := range phy.Bands {
			var freqs []int
			for f := range b.FrequencyFlags {
				if f > 2400 && f < 2500 {
					has24GHz = true
				} else if f > 5000 && f < 6000 {
					// 5GHz
				} else if f > 6000 && f < 7000 {
					// 6GHz
				} else {
					testing.ContextLogf(ctx, "Unknown frequency %d", f)
				}
				if len(b.FrequencyFlags[f]) == 1 && b.FrequencyFlags[f][0] == "disabled" {
					continue
				}
				freqs = append(freqs, f)
			}
			testing.ContextLogf(ctx, "Band %d Frequencies: %s", b.Num, fmt.Sprint(freqs))
		}
		// Minimum requirement for MVP
		return has24GHz
	}

	checkConcurrency := func(phy *iw.Phy) int {
		maxConcurrent := 0
		max := func(a, b int) int {
			if a < b {
				return b
			}
			return a
		}
		for _, ic := range phy.IfaceCombinations {
			// Check for AP >= 1 in limits.
			apFound := false
			for _, il := range ic.IfaceLimits {
				for _, it := range il.IfaceTypes {
					if it == iw.IfTypeAP && il.MaxCount >= 1 {
						apFound = true
					}
				}
			}
			if !apFound {
				continue
			}
			maxConcurrent = max(maxConcurrent, ic.MaxTotal)
		}
		return maxConcurrent
	}

	// We're using phys[0] basing on two assumptions:
	// 1. All phys of the same device will have the same capabilities.
	// 2. We support only one WiFi device per DUT.
	if !checkMode(phys[0], iw.IfTypeAP) {
		s.Error("Device does not declare AP-mode support")
	}

	if !checkBands(phys[0]) {
		s.Error("Device does not declare all expected bands")
	}

	if phys[0].MaxSTAs > 0 {
		testing.ContextLogf(ctx, "Max concurrent clients: %d", phys[0].MaxSTAs)
	}

	maxConcurrent := checkConcurrency(phys[0])
	if maxConcurrent > 1 {
		testing.ContextLogf(ctx, "Device supports AP in Concurrent Mode, limit %d", maxConcurrent)
	} else {
		testing.ContextLog(ctx, "Device does not declare support for AP in Concurrent Mode")
	}
}
