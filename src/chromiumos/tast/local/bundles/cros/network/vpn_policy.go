// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/bundles/cros/network/vpn"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VPNPolicy,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Test that VPN can correctly be configured from device and user policy",
		Contacts: []string{
			"taoyl@google.com",
			"cros-networking@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromeEnrolledLoggedIn",
	})
}

func VPNPolicy(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	deviceProfileServiceGUID := "Device Policy L2TPIPSec-VPN"
	userProfileServiceGUID := "User Policy L2TPIPSec-VPN"

	m, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to create shill manager proxy: ", err)
	}

	server, err := vpn.StartL2TPIPsecServer(ctx, vpn.AuthTypePSK, false, false)
	if err != nil {
		s.Fatal("Failed to start VPN server: ", err)
	}
	defer server.Exit(ctx)

	testing.ContextLog(ctx, "VPN server started as ", server.UnderlayIP)

	vpnONC := &policy.ONCVPN{
		AutoConnect: false,
		Host:        server.UnderlayIP,
		Type:        "L2TP-IPsec",
		L2TP: &policy.ONCL2TP{
			Username: "chapuser",
			Password: "chapsecret",
		},
		IPsec: &policy.ONCIPsec{
			AuthenticationType: "PSK",
			IKEVersion:         1,
			PSK:                "preshared-key",
		},
	}

	userNetPolicy := &policy.OpenNetworkConfiguration{
		Val: &policy.ONC{
			NetworkConfigurations: []*policy.ONCNetworkConfiguration{
				{
					GUID: userProfileServiceGUID,
					Name: "User Policy L2TPIPSec",
					Type: "VPN",
					VPN:  vpnONC,
				},
			},
		},
	}

	deviceNetPolicy := &policy.DeviceOpenNetworkConfiguration{
		Val: &policy.ONC{
			NetworkConfigurations: []*policy.ONCNetworkConfiguration{
				{
					GUID: deviceProfileServiceGUID,
					Name: "Device Policy L2TPIPSec",
					Type: "VPN",
					VPN:  vpnONC,
				},
			},
		},
	}

	for _, tc := range []struct {
		subtest string
		policy  []policy.Policy
		guid    string
	}{
		{
			subtest: "device",
			policy:  []policy.Policy{deviceNetPolicy},
			guid:    deviceProfileServiceGUID,
		},
		{
			subtest: "user",
			policy:  []policy.Policy{userNetPolicy},
			guid:    userProfileServiceGUID,
		},
	} {
		s.Run(ctx, tc.subtest, func(ctx context.Context, s *testing.State) {
			if err := policyutil.ServeAndRefresh(ctx, fdms, cr, tc.policy); err != nil {
				s.Fatalf("Failed to update %s policy: %v", tc.subtest, err)
			}

			if err := vpn.VerifyVPNProfile(ctx, m, tc.guid, true); err != nil {
				s.Errorf("Verifying %s profile defined VPN failed: %v", tc.subtest, err)
			}
		})
	}
}
