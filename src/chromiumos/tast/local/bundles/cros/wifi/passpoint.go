// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"chromiumos/tast/local/bundles/cros/wifi/hwsim"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Passpoint,
		Desc: "Passpoint functionnal tests",
		Contacts: []string{
			"damiendejean@chromium.org", // Test author
			"cros-networking@google.com",
		},
		Fixture:      "shillSimulatedWiFi",
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"wifi", "shill-wifi"},
	})
}

func Passpoint(fullCtx context.Context, s *testing.State) {
	// Obtain the test interfaces from the fixture.
	sim := s.FixtValue().(*hwsim.ShillSimulatedWiFi)
	if len(sim.AP) == 0 {
		s.Fatal("No test interfaces")
	}

	// TODO(b/162258594) implement Passpoint test.
}
