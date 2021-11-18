// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
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

			if err := verifyVPNProfile(ctx, m, tc.guid); err != nil {
				s.Errorf("Verifying %s profile defined VPN failed: %v", tc.subtest, err)
			}
		})
	}
}

func verifyVPNProfile(ctx context.Context, m *shill.Manager, serviceGUID string) error {
	testing.ContextLog(ctx, "Trying to find service with guid ", serviceGUID)

	findServiceProps := make(map[string]interface{})
	findServiceProps["GUID"] = serviceGUID
	findServiceProps["Type"] = "vpn"
	service, err := m.WaitForServiceProperties(ctx, findServiceProps, 5*time.Second)
	if err != nil {
		testing.ContextLog(ctx, "Cannot find service matching guid ", serviceGUID)
		return err
	}
	testing.ContextLogf(ctx, "Found service %v matching guid %s, connecting", service, serviceGUID)

	pw, err := service.CreateWatcher(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create watcher")
	}
	defer pw.Close(ctx)
	if err = service.Connect(ctx); err != nil {
		return errors.Wrapf(err, "failed to connect the service %v", service)
	}
	defer func() {
		if err = service.Disconnect(ctx); err != nil {
			testing.ContextLog(ctx, "Failed to disconnect service ", service)
		}
	}()

	timeoutCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	state, err := pw.ExpectIn(timeoutCtx, shillconst.ServicePropertyState, append(shillconst.ServiceConnectedStates, shillconst.ServiceStateFailure))
	if err != nil {
		return err
	}

	if state == shillconst.ServiceStateFailure {
		return errors.Errorf("service %v became failure state", service)
	}
	return nil
}
