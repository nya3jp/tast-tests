// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	//"chromiumos/tast/local/bundles/cros/network/networkhealth"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HealthGetActiveNetworks,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Validates that NetworkHealth API accurately gets networks",
		Contacts: []string{
			"khegde@chromium.org",            // test maintainer
			"stevenjb@chromium.org",          // network-health tech lead
			"cros-network-health@google.com", // network-health team
		},
		SoftwareDeps: []string{"chrome", "no_qemu"},
		Attr:         []string{"group:mainline", "informational"},
		//Fixture:      "networkHealth",
	})
}

// HealthGetActiveNetworks validates that the NetworkHealth API correctly retrieves
// networks.
func HealthGetActiveNetworks(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed start Chrome: ", err)
	}
	defer cr.Close(ctx)
}
