// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"chromiumos/tast/local/network/iw"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: HeCaps,
		Desc: "Verifies HE-MAC supported DUT actually supports Wifi HE protocols",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:         []string{"group:mainline", "group:wificell", "wificell_func"},
		SoftwareDeps: []string{"wifi", "shill-wifi"},
		HardwareDeps: hwdep.D(hwdep.Wifi80211ax()),
	})
}

func HeCaps(ctx context.Context, s *testing.State) {
	iwr := iw.NewLocalRunner()

	// Get the information of the WLAN device.
	res, err := iwr.ListPhys(ctx)
	if err != nil {
		s.Fatal("ListPhys failed: ", err)
	}
	if len(res) == 0 {
		s.Fatal("Expect at least one wireless phy; found nothing")
	}
	if !res[0].SupportHE {
		s.Error("Device doesn't support HE-MAC capabilities")
	}
	if !res[0].SupportHE80 {
		s.Error("Device doesn't support 5ghz HE-MAC capabilities")
	}

}
