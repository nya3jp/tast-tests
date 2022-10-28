// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/vpn"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     HostVPNAlwaysOn,
		Desc:     "OpenVPN client can be configured as always-on VPN and connects automatically",
		Contacts: []string{"chuweih@google.com", "cros-networking@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Fixture:  "vpnShillReset",
		Params: []testing.Param{
			{
				Name: "strict_mode",
				Val:  "strict",
			},
			{
				Name: "best_effort_mode",
				Val:  "best-effort",
			},
		},
	})
}

// HostVPNAlwaysOn Sets up an openVPN and check if this VPN can be configured
// as always-on VPN and connects automatically.
func HostVPNAlwaysOn(ctx context.Context, s *testing.State) {
	m, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to create shill manager proxy: ", err)
	}

	// If the main body of the test times out, we still want to reserve a
	// few seconds to allow for our cleanup code to run.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Set up an test profile and pop it out on stack after test is finished.
	popFunc, err := m.PushTestProfile(ctx)
	if err != nil {
		s.Fatal("Failed to push test profile: ", err)
	}
	defer popFunc()

	// Set up an openVPN connection.
	config := vpn.Config{
		Type:           vpn.TypeOpenVPN,
		AuthType:       vpn.AuthTypeCert,
		OpenVPNTLSAuth: true,
	}
	config.CertVals = s.FixtValue().(vpn.FixtureEnv).CertVals
	conn, err := vpn.NewConnection(ctx, config)
	if err != nil {
		s.Fatal("Failed to create connection object: ", err)
	}

	defer func() {
		if err := conn.Cleanup(cleanupCtx); err != nil {
			s.Error("Failed to clean up connection: ", err)
		}
	}()

	if err := conn.SetUp(ctx); err != nil {
		s.Fatal("Failed to setup VPN server: ", err)
	}

	// Use set up openVPN as service and set it in tested mode.
	profile, err := m.ActiveProfile(ctx)
	if err != nil {
		s.Fatal("Failed to get active profile: ", err)
	}
	propHolder := profile.PropertyHolder
	vpnMode := s.Param().(string)
	propHolder.SetProperty(ctx, "AlwaysOnVpnService", &conn.ServicePath)
	propHolder.SetProperty(ctx, "AlwaysOnVpnMode", vpnMode)

	// Check if always on VPN is set in correct mode.
	props, err := propHolder.GetProperties(ctx)
	if err != nil {
		s.Error("Failed to get props: ", err)
	}
	curMode, _ := props.GetString("AlwaysOnVpnMode")
	if curMode != vpnMode {
		s.Errorf("Current VPN mode is %v, want: %v", curMode, vpnMode)
	}

	// Check if VPN can be automatically connected in 10 seconds.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		connected, err := conn.IsConnected(ctx)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get connection state"))
		}
		if connected {
			return nil
		}
		return errors.Wrap(err, "not connected")
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Error(err, "VPN cannot connect")
	}
}
