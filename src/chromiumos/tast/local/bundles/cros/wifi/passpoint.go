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
		},
		Fixture: "shillSimulatedWifi",
	})
}

func Passpoint(fullCtx context.Context, s *testing.State) {
	// Obtain the test interfaces from the fixture.
	ifaces := s.FixtValue().(*hwsim.FixtureIfaces)
	if len(ifaces.AP) == 0 {
		s.Fatal("No test interfaces")
	}

	// Create a Passpoint compatible access point on one of the test interfaces.
	certs := certificate.TestCert1()
	conf := passpoint.APConf{
		SSID:              "passpoint",
		Auth:              passpoint.AuthTTLS,
		Identity:          "user",
		Password:          "password",
		Cert:              &certs,
		Domain:            "passpoint.example.com",
		Realms:            []string{"passpoint.example.com"},
		RoamingConsortium: "2233445566",
	}
	ap := hostapd.Server{
		Iface:  ifaces.AP[0],
		OutDir: s.OutDir(),
		Conf:   conf,
	}
	err := ap.Start(fullCtx)
	if err != nil {
		s.Fatal("Failed to create access point: ", err)
	}
	defer ap.Stop(fullCtx)

	// TODO(b/162258594) implement Passpoint test.
}
