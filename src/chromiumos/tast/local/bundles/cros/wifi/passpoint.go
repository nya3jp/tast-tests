// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/local/bundles/cros/wifi/hwsim"
	"chromiumos/tast/local/bundles/cros/wifi/passpoint"
	"chromiumos/tast/local/hostapd"
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

	// Create a Passpoint compatible access point on one of the test interfaces.
	certs := certificate.TestCert1()
	conf := passpoint.NewAPConf(
		"passpoint",
		passpoint.AuthTTLS,
		"user",
		"password",
		&certs,
		"passpoint.example.com",
		[]string{"passpoint.example.com"},
		"2233445566",
	)
	ap := hostapd.NewServer(sim.AP[0], s.OutDir(), conf)
	if err := ap.Start(fullCtx); err != nil {
		s.Fatal("Failed to create access point: ", err)
	}
	defer ap.Stop(fullCtx)

	// TODO(b/162258594) implement Passpoint test.
}
