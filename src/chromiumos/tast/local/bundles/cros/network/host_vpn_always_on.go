// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/network/vpn"
	"chromiumos/tast/local/network/routing"
	"chromiumos/tast/local/network/virtualnet"
	"chromiumos/tast/local/network/virtualnet/env"
	"chromiumos/tast/local/network/virtualnet/subnet"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

type hostVPNAlwaysOnTestCase struct {
	// mode of Always-on VPN we want to test.
	mode string
	// configurarion of host VPN.
	config vpn.Config
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         HostVPNAlwaysOn,
		Desc:         "Host VPN client can be configured as always-on VPN and connected automatically",
		Contacts:     []string{"chuweih@google.com", "cros-networking@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "vpnShillReset",
		LacrosStatus: testing.LacrosVariantUnneeded,
		Params: []testing.Param{
			{
				Name: "strict_mode_ikev2",
				Val: hostVPNAlwaysOnTestCase{
					mode: shillconst.AlwaysOnVPNModeStrict,
					config: vpn.Config{
						Type:     vpn.TypeIKEv2,
						AuthType: vpn.AuthTypePSK,
					},
				},
				ExtraSoftwareDeps: []string{"ikev2"},
			},
			{
				Name: "strict_mode_l2tp_ipsec_psk",
				Val: hostVPNAlwaysOnTestCase{
					mode: shillconst.AlwaysOnVPNModeStrict,
					config: vpn.Config{
						Type:     vpn.TypeL2TPIPsec,
						AuthType: vpn.AuthTypePSK,
					},
				},
			},
			{
				Name: "best_effort_mode_ikev2",
				Val: hostVPNAlwaysOnTestCase{
					mode: shillconst.AlwaysOnVPNModeBestEffort,
					config: vpn.Config{
						Type:     vpn.TypeIKEv2,
						AuthType: vpn.AuthTypePSK,
					},
				},
				ExtraSoftwareDeps: []string{"ikev2"},
			},
			{
				Name: "best_effort_mode_l2tp_ipsec_psk",
				Val: hostVPNAlwaysOnTestCase{
					mode: shillconst.AlwaysOnVPNModeBestEffort,
					config: vpn.Config{
						Type:     vpn.TypeL2TPIPsec,
						AuthType: vpn.AuthTypePSK,
					},
				},
			},
		},
	})
}

// HostVPNAlwaysOn sets up an host VPN and checks if this VPN can be configured
// as always-on VPN, connects automatically and routing is correct when VPN service
// is not available.
//
// In this test, we set up the network as follows:
//
//	veth0 --+-- test router --+-- physical server (used to test if system/user traffic is blocked)
//	            DHCP server   |
//	                          +-- virtual server (used to configure Always-on VPN)
func HostVPNAlwaysOn(ctx context.Context, s *testing.State) {
	// If the main body of the test times out, we still want to reserve a
	// few seconds to allow for our cleanup code to run.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	m, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to create shill manager proxy: ", err)
	}

	if err := m.SetProperty(ctx, shillconst.ProfilePropertyCheckPortalList, "wifi,cellular"); err != nil {
		s.Fatal("Failed to disable portal detection on ethernet: ", err)
	}
	defer func() {
		if err := m.SetProperty(cleanupCtx, shillconst.ProfilePropertyCheckPortalList, "ethernet,wifi,cellular"); err != nil {
			s.Fatal("Failed to restore portal detection on ethernet: ", err)
		}
	}()

	// Set up an test profile and pop it out on stack after test is finished.
	popFunc, err := m.PushTestProfile(ctx)
	if err != nil {
		s.Fatal("Failed to push test profile: ", err)
	}
	defer popFunc()

	pool := subnet.NewPool()

	// Setup a router and connect 2 servers.
	svc, rt, svr, err := virtualnet.CreateRouterServerEnv(ctx, m, pool, virtualnet.EnvOptions{
		Priority:   10,
		EnableDHCP: true,
	})
	if err != nil {
		s.Fatal("Failed to create router env: ", err)
	}
	defer rt.Cleanup(cleanupCtx)

	vsvr := env.New("vserver")
	if err := vsvr.SetUp(ctx); err != nil {
		s.Fatal("Failed to setup server: ", err)
	}
	if err := vsvr.ConnectToRouterWithPool(ctx, rt, pool); err != nil {
		s.Fatal("Failed to connect server to router: ", err)
	}
	if err := svc.WaitForProperty(ctx, shillconst.ServicePropertyState, shillconst.ServiceStateOnline, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for service online: ", err)
	}
	addrs, err := svr.WaitForVethInAddrs(ctx, true, false)
	if err != nil {
		s.Fatal("Failed to get server addrs: ", err)
	}
	physicalAddr := addrs.IPv4Addr.String()

	// Establish a VPN on one of the servers.
	config := s.Param().(hostVPNAlwaysOnTestCase).config
	config.CertVals = s.FixtValue().(vpn.FixtureEnv).CertVals
	conn, err := vpn.NewConnectionWithEnvs(ctx, config, vsvr, nil)
	if err != nil {
		s.Fatal("Failed to connect vpn: ", err)
	}
	defer conn.Cleanup(cleanupCtx)
	if err := conn.SetUp(ctx); err != nil {
		s.Fatal("Failed to setup vpn: ", err)
	}

	// Use set up host VPN as service and change the Always-on VPN mode.
	profile, err := m.ActiveProfile(ctx)
	if err != nil {
		s.Fatal("Failed to get active profile: ", err)
	}
	vpnMode := s.Param().(hostVPNAlwaysOnTestCase).mode
	if err := profile.SetProperty(ctx, shillconst.ProfilePropertyAlwaysOnVPNServive, conn.Service().DBusObject.ObjectPath()); err != nil {
		s.Fatal("Failed to set Always-on VPN service: ", err)
	}
	if err := profile.SetProperty(ctx, shillconst.ProfilePropertyAlwaysOnVPNMode, vpnMode); err != nil {
		s.Fatal("Failed to set Always-on VPN mode: ", err)
	}

	// Check if always on VPN is set in correct mode.
	props, err := profile.GetProperties(ctx)
	if err != nil {
		s.Fatal("Failed to get props: ", err)
	}
	if curMode, err := props.GetString(shillconst.ProfilePropertyAlwaysOnVPNMode); err != nil {
		s.Fatal("Failed to get Always-on VPN mode: ", err)
	} else if curMode != vpnMode {
		s.Errorf("Current Always-on VPN mode is %v, want: %v", curMode, vpnMode)
	}

	// Check if VPN can be automatically connected.
	if err := conn.Service().WaitForConnectedOrError(ctx); err != nil {
		s.Fatal("Failed to wait for VPN connected automatically: ", err)
	}

	// Disconnect VPN and block all traffic in VPN netNS, make sure VPN does not re-connect.
	if err := conn.Disconnect(ctx); err != nil {
		s.Fatal("Failed to disconnect from VPN: ", err)
	}

	cmd := []string{"iptables", "-w", "-I", "INPUT", "-j", "DROP"}
	if err := vsvr.RunWithoutChroot(ctx, cmd...); err != nil {
		s.Fatal("Failed to block all traffic for VPN: ", err)
	}

	// Test system traffic is not blocked if the VPN is not connectble for both modes.
	if err := routing.ExpectPingSuccessWithTimeout(ctx, physicalAddr, "root", 10*time.Second); err != nil {
		s.Errorf("User %s failed to ping %v: %v", "root", physicalAddr, err)
	}

	// Test user traffic is blocked by strict mode but not best effort mode.
	const user = "chronos"
	switch vpnMode {
	case shillconst.AlwaysOnVPNModeStrict:
		if err := routing.ExpectPingFailure(ctx, physicalAddr, user); err != nil {
			s.Errorf("User %s succeeded to ping %v in strict mode: %v", user, physicalAddr, err)
		}
	case shillconst.AlwaysOnVPNModeBestEffort:
		if err := routing.ExpectPingSuccessWithTimeout(ctx, physicalAddr, user, 10*time.Second); err != nil {
			s.Errorf("User %s failed to ping %v: %v", user, physicalAddr, err)
		}
	}
}
