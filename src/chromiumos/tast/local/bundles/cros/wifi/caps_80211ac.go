// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"io/ioutil"
	"path/filepath"

	"chromiumos/tast/local/network/iw"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Caps80211ac,
		Desc: "Verifies DUT supports required 802.11ac capabilities",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:         []string{"group:mainline", "group:wificell", "wificell_func"},
		SoftwareDeps: []string{"wifi"},
		HardwareDeps: hwdep.D(hwdep.Wifi80211ac()),
	})
}

func Caps80211ac(ctx context.Context, s *testing.State) {
	iwr := iw.NewLocalRunner()
	res, out, err := iwr.ListPhys(ctx)
	if err != nil {
		s.Fatal("ListPhys failed: ", err)
	}
	if len(res) == 0 {
		s.Fatal("Expect at least one wireless phy; found nothing")
	}
	// Save `iw list` text to log file.
	ioutil.WriteFile(filepath.Join(s.OutDir(), "iw_list"), out, 0644)

	if !res[0].SupportVHT {
		s.Error("Device doesn't support VHT")
	}
	if !res[0].SupportVHT80SGI {
		s.Error("Device doesn't support VHT80 short guard interval")
	}
}
