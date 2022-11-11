// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	pp "chromiumos/system_api/patchpanel_proto"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	patchpanel "chromiumos/tast/local/network/patchpanel_client"
	"chromiumos/tast/local/network/routing"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

type ipv6TestParams struct {
	v6Only bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         IPv6Connectivity,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks IPv6 connectivity inside ARC",
		Contacts:     []string{"taoyl@google.com", "cros-networking@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"arc", "chrome"},
		Timeout:      4 * time.Minute,
		Fixture:      "arcBooted",
		Params: []testing.Param{{
			Val: ipv6TestParams{},
		}, {
			Name: "v6only",
			Val: ipv6TestParams{
				v6Only: true,
			},
		}},
	})
}

func IPv6Connectivity(ctx context.Context, s *testing.State) {
	a := s.FixtValue().(*arc.PreData).ARC
	v6only := s.Param().(ipv6TestParams).v6Only

	// Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	shillManager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to create shill client: ", err)
	}
	hiddenIfs, err := arc.HideUnusedEthernet(ctx, shillManager)
	if err != nil {
		s.Fatal("Failed to hide unused ethernet: ", err)
	}
	defer arc.RestoreHiddenEthernet(cleanupCtx, shillManager, hiddenIfs)

	// Set up test topology
	testEnv := routing.NewSimpleNetworkEnv(!v6only, true, !v6only, true)
	if err := testEnv.SetUp(ctx); err != nil {
		s.Fatal("Failed to set up routing test env: ", err)
	}
	defer func(ctx context.Context) {
		if err := testEnv.TearDown(ctx); err != nil {
			s.Error("Failed to tear down routing test env: ", err)
		}
	}(cleanupCtx)

	// Wait for online and verify topology in host
	if err := testEnv.ShillService.WaitForProperty(ctx, shillconst.ServicePropertyState, shillconst.ServiceStateOnline, 10*time.Second); err != nil {
		s.Error("Failed to wait for service online: ", err)
	}

	routerAddrs, err := testEnv.Router.WaitForVethInAddrs(ctx, false, true)
	if err != nil {
		s.Fatal("Failed to get inner addrs from router env: ", err)
	}
	serverAddrs, err := testEnv.Server.WaitForVethInAddrs(ctx, false, true)
	if err != nil {
		s.Fatal("Failed to get inner addrs from server env: ", err)
	}
	var pingAddrs []string
	for _, ip := range routerAddrs.IPv6Addrs {
		pingAddrs = append(pingAddrs, ip.String())
	}
	for _, ip := range serverAddrs.IPv6Addrs {
		pingAddrs = append(pingAddrs, ip.String())
	}
	pingAddrs = append(pingAddrs, "v6.foo.bar")

	for _, target := range pingAddrs {
		if err := routing.ExpectPingSuccessWithTimeout(ctx, target, "chronos", 10*time.Second); err != nil {
			s.Errorf("Network verification failed: %v is not reachable as user %s on host: %v", target, "chronos", err)
		}
	}

	vethName := testEnv.Router.VethOutName
	arcIfname, err := getARCInterfaceName(ctx, vethName)
	if err != nil {
		s.Fatalf("Failed to get ARC interface name corresponding to %s: %v", vethName, err)
	}

	// Check if testEnv prefix propagated into ARC, and log it for debugging.
	const addressPollTimeout = 5 * time.Second
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := a.Command(ctx, "/system/bin/ip", "-6", "addr", "show", "scope", "global", "dev", arcIfname).Output(testexec.DumpLogOnError)
		if err != nil {
			return err
		}
		if len(out) == 0 {
			return errors.New("no global IPv6 address is configured")
		}
		testing.ContextLog(ctx, "ARC address information: ", string(out))
		return nil
	}, &testing.PollOptions{Timeout: addressPollTimeout}); err != nil {
		s.Fatalf("Failed to get global IPv6 address on %s in ARC: %v", arcIfname, err)
	}

	// ping virtual router address and virtual server address from ARC.
	for _, target := range pingAddrs {
		if err := arc.ExpectPingSuccess(ctx, a, arcIfname, target); err != nil {
			s.Errorf("Failed to ping %s from ARC over %q: %v", target, arcIfname, err)
		}
		if err := arc.ExpectPingSuccess(ctx, a, "", target); err != nil {
			s.Errorf("Failed to ping %s from ARC over default network: %v", target, err)
		}
	}
}

func getARCInterfaceName(ctx context.Context, hostIfname string) (string, error) {
	pc, err := patchpanel.New(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to create patchpanel client")
	}
	response, err := pc.GetDevices(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to get patchpanel devices")
	}
	for _, device := range response.Devices {
		if (device.GuestType == pp.NetworkDevice_ARCVM || device.GuestType == pp.NetworkDevice_ARC) && device.PhysIfname == hostIfname {
			return device.GuestIfname, nil
		}
	}
	return "", errors.Errorf("no ARC device matching %s is found", hostIfname)
}
