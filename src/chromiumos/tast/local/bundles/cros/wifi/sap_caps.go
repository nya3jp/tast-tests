// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
		Func: SAPCaps,
		Desc: "Verifies DUT supports SoftAP and a minimum set of required protocols",
		Contacts: []string{
			"jintaolin@chromium.org",          // Test author
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:         []string{"group:wificell", "wificell_func", "wificell_unstable"},
		SoftwareDeps: []string{"wifi"},
		HardwareDeps: hwdep.D(hwdep.WifiSAP()),
	})
}

func SAPCaps(ctx context.Context, s *testing.State) {
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

	apSupported := false
	for _, mode := range res[0].Modes {
		if mode == "AP" {
			apSupported = true
		}
	}
	if !apSupported {
		s.Error("AP mode not supported")
	}

	if res[0].MaxSTAs < 16 {
		s.Error("Less than 16 associated STAs are supported in AP mode: ", res[0].MaxSTAs)
	}

	if res[0].SupportHESTA {
		if !res[0].SupportHEAP {
			s.Error("Device doesn't support HE in AP mode")
		}
	}

	if res[0].SupportHE40HE80STA {
		if !res[0].SupportHE40HE80AP {
			s.Error("Device doesn't support 5ghz HE in AP mode")
		}
	}
}
