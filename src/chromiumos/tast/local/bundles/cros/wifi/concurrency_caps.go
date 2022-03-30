// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	commonIw "chromiumos/tast/common/network/iw"
	"chromiumos/tast/local/network/iw"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ConcurrencyCaps,
		Desc: "Records DUT's concurrency capabilities",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:         []string{"group:mainline", "group:wificell", "wificell_unstable", "informational"},
		SoftwareDeps: []string{"wifi", "shill-wifi"},
	})
}

func ConcurrencyCaps(ctx context.Context, s *testing.State) {
	iwr := iw.NewLocalRunner()

	res, err := iwr.ListPhys(ctx)
	if err != nil {
		s.Fatal("ListPhys failed: ", err)
	}
	if len(res) == 0 {
		s.Fatal("Expect at least one wireless phy; found nothing")
	}

	supportsAPSTA := false
	supportsAPSTAMultiChannel := false
	supportsP2PSTA := false
	supportsP2PSTAMultiChannel := false
	for _, combination := range res[0].IfaceCombinations {
		if supportsConcurrency(combination, []string{"managed", "AP"}) {
			supportsAPSTA = true
			if combination.MaxChannels >= 2 {
				supportsAPSTAMultiChannel = true
			}
		}
		if supportsConcurrency(combination, []string{"managed", "P2P-GO"}) &&
			supportsConcurrency(combination, []string{"managed", "P2P-client"}) &&
			supportsConcurrency(combination, []string{"managed", "P2P-device"}) {
			supportsP2PSTA = true
			if combination.MaxChannels >= 2 {
				supportsP2PSTAMultiChannel = true
			}
		}
	}
	s.Logf("Supports AP/STA concurrency: %t", supportsAPSTA)
	s.Logf("Supports AP/STA Multi-channel concurrency: %t", supportsAPSTAMultiChannel)
	s.Logf("Supports P2P/STA concurrency: %t", supportsP2PSTA)
	s.Logf("Supports P2P/STA Multi-channel concurrency: %t", supportsP2PSTAMultiChannel)
}

// supportsConcurrency returns true if the given InterfaceCombination is
// capable of handling all of the given interfaces concurrently, and false
// otherwise.
func supportsConcurrency(combination commonIw.IfaceCombination, concurrentIfaces []string) bool {
	if combination.MaxTotal < len(concurrentIfaces) {
		return false
	}
	var availableIfaceCounts []int
	for _, limit := range combination.IfaceLimits {
		availableIfaceCounts = append(availableIfaceCounts, limit.MaxCount)
	}
	for _, concurrentIface := range concurrentIfaces {
		ifaceFound := false
		for i, limit := range combination.IfaceLimits {
			for _, iface := range limit.IfaceTypes {
				if iface == concurrentIface {
					ifaceFound = true
					availableIfaceCounts[i]--
					if availableIfaceCounts[i] < 0 {
						return false
					}
				}
			}
		}
		if !ifaceFound {
			return false
		}
	}
	return true
}
