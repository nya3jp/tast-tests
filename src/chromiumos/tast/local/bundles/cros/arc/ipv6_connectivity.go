// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"net"
	"time"

	pp "chromiumos/system_api/patchpanel_proto"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	patchpanel "chromiumos/tast/local/network/patchpanel_client"
	"chromiumos/tast/local/network/routing"
	"chromiumos/tast/testing"
)

// NetDeviceType indicates the type of netdevice between host and ARC.
type netDeviceType string

const (
	// Veth used by ARC container.
	netDeviceTypeVeth netDeviceType = "veth"
	// VMtap used by ARCVM.
	netDeviceTypeVmtap netDeviceType = "vmtap"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         IPv6Connectivity,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks IPv6 connectivity inside ARC",
		Contacts:     []string{"taoyl@google.com", "cros-networking@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
		Fixture:      "arcBooted",
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			Val:               netDeviceTypeVeth,
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val:               netDeviceTypeVmtap,
		}},
	})
}

func IPv6Connectivity(ctx context.Context, s *testing.State) {
	a := s.FixtValue().(*arc.PreData).ARC

	// Use a shortened context for test operations to reserve time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	testEnv := routing.NewTestEnv()
	if err := testEnv.SetUp(ctx); err != nil {
		s.Fatal("Failed to set up routing test env: ", err)
	}
	defer func(ctx context.Context) {
		if err := testEnv.TearDown(ctx); err != nil {
			s.Error("Failed to tear down routing test env: ", err)
		}
	}(cleanupCtx)

	vethName := testEnv.BaseRouter.VethOutName
	arcIfname, err := getARCInterfaceName(ctx, s.Param().(netDeviceType), vethName)
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
	routerAddrs, err := testEnv.BaseRouter.WaitForVethInAddrs(ctx, false, true)
	if err != nil {
		s.Fatal("Failed to get inner addrs from router env: ", err)
	}
	serverAddrs, err := testEnv.BaseServer.WaitForVethInAddrs(ctx, false, true)
	if err != nil {
		s.Fatal("Failed to get inner addrs from server env: ", err)
	}

	var pingAddrs []net.IP
	pingAddrs = append(pingAddrs, routerAddrs.IPv6Addrs...)
	pingAddrs = append(pingAddrs, serverAddrs.IPv6Addrs...)
	for _, ip := range pingAddrs {
		if err := arc.ExpectPingSuccess(ctx, a, arcIfname, ip.String()); err != nil {
			s.Fatalf("Failed to ping %s from ARC over %q: %v", ip.String(), arcIfname, err)
		}
		if err := arc.ExpectPingSuccess(ctx, a, "", ip.String()); err != nil {
			s.Fatalf("Failed to ping %s from ARC over default network: %v", ip.String(), err)
		}
	}

}

func getARCInterfaceName(ctx context.Context, netdevType netDeviceType, hostIfname string) (string, error) {
	if netdevType == netDeviceTypeVeth {
		// In Veth setup the interface name within ARC is same with host interface name
		return hostIfname, nil
	} else if netdevType == netDeviceTypeVmtap {
		pc, err := patchpanel.New(ctx)
		if err != nil {
			return "", errors.Wrap(err, "failed to create patchpanel client")
		}

		response, err := pc.GetDevices(ctx)
		if err != nil {
			return "", errors.Wrap(err, "failed to get patchpanel devices")
		}
		for _, device := range response.Devices {
			if device.GuestType == pp.NetworkDevice_ARCVM && device.PhysIfname == hostIfname {
				return device.GuestIfname, nil
			}
		}
	}
	return "", errors.Errorf("no device matching %s is found", hostIfname)
}
