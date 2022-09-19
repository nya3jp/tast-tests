// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"io/ioutil"
	"path/filepath"

	"chromiumos/tast/common/network/iw"
	localIw "chromiumos/tast/local/network/iw"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ConcurrencyCaps,
		Desc: "Records DUT's concurrency capabilities",
		Contacts: []string{
			"kglund@google.com",               // Test author
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:         []string{"group:mainline", "group:wificell", "wificell_unstable", "informational"},
		SoftwareDeps: []string{"wifi", "shill-wifi"},
	})
}

func ConcurrencyCaps(ctx context.Context, s *testing.State) {
	iwr := localIw.NewLocalRunner()

	res, out, err := iwr.ListPhys(ctx)
	if err != nil {
		s.Fatal("ListPhys failed: ", err)
	}
	if len(res) == 0 {
		s.Fatal("Expect at least one wireless phy; found nothing")
	}
	// Save `iw list` text to log file.
	ioutil.WriteFile(filepath.Join(s.OutDir(), "iw_list"), out, 0644)

	supportsAPSTA := false
	supportsAPSTAMultiChannel := false
	supportsP2PSTA := false
	supportsP2PSTAMultiChannel := false
	for _, combination := range res[0].IfaceCombinations {
		if supportsConcurrency(combination, []iw.IfType{iw.IfTypeManaged, iw.IfTypeAP}) {
			supportsAPSTA = true
			if combination.MaxChannels >= 2 {
				supportsAPSTAMultiChannel = true
			}
		}
		if supportsConcurrency(combination, []iw.IfType{iw.IfTypeManaged, iw.IfTypeP2PGO}) &&
			supportsConcurrency(combination, []iw.IfType{iw.IfTypeManaged, iw.IfTypeP2PClient}) &&
			supportsConcurrency(combination, []iw.IfType{iw.IfTypeManaged, iw.IfTypeP2PDevice}) {
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
func supportsConcurrency(combination iw.IfaceCombination, concurrentIfaces []iw.IfType) bool {
	if combination.MaxTotal < len(concurrentIfaces) {
		return false
	}

	ifaceCounts := make([]int, len(combination.IfaceLimits))
	for _, concurrentIface := range concurrentIfaces {
		ifaceFound := false
		for i, limit := range combination.IfaceLimits {
			for _, iface := range limit.IfaceTypes {
				if iface == concurrentIface {
					ifaceFound = true
					ifaceCounts[i]++
					if ifaceCounts[i] > limit.MaxCount {
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
