// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"encoding/binary"
	"net"
	"strings"
	"time"

	pp "chromiumos/system_api/patchpanel_proto"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/network"
	patchpanel "chromiumos/tast/local/network/patchpanel_client"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MultiNetworkingConnectivity,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks connectivity while multi-networking is enabled",
		Contacts:     []string{"jasongustaman@google.com", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Params: []testing.Param{
			{
				ExtraSoftwareDeps: []string{"android_p"},
			}, {
				Name:              "vm",
				ExtraSoftwareDeps: []string{"android_vm"},
			},
		},
	})
}

func MultiNetworkingConnectivity(ctx context.Context, s *testing.State) {
	const (
		// timeout defines the maximum allowed time for connectivity check.
		timeout = 10 * time.Second
	)

	// Get the ARC instance.
	a := s.FixtValue().(*arc.PreData).ARC

	pc, err := patchpanel.New(ctx)
	if err != nil {
		s.Fatal("Failed to create patchpanel client: ", err)
	}

	// Get all patchpanel managed devices.
	response, err := pc.GetDevices(ctx)
	if err != nil {
		testing.ContextLog(ctx, "Failed to get patchpanel devices: ", err)
	}

	// Ensure that inbound and outbound networking works for each network interface inside ARC for each host's physical interfaces.
	for _, device := range response.Devices {
		if device.GuestType != pp.NetworkDevice_ARC && device.GuestType != pp.NetworkDevice_ARCVM {
			continue
		}

		if ifup, err := interfaceUp(ctx, device.PhysIfname); err != nil {
			s.Logf("Failed checking if %s is up: %s", device.PhysIfname, err)
			continue
		} else if !ifup {
			continue
		}

		// Get the first hop from the host physical interface to Google DNS server.
		gateway, err := network.Gateway(ctx, device.PhysIfname)
		if err != nil {
			s.Errorf("Failed to get gateway address for interface %s: %s", device.PhysIfname, err)
			continue
		}

		// This code tests outbound network from within Android (ARC).
		// It fetches ARC network interfaces, and for each of the interface,
		// test if a ping to the gateway.
		s.Logf("Pinging from ARC interface %s through host interface %s", device.GuestIfname, device.PhysIfname)
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			out, err := a.Command(ctx, "dumpsys", "wifi", "tools", "reach", device.GuestIfname, gateway).Output()
			if err != nil {
				return err
			}
			if !strings.Contains(string(out), gateway+": reachable") {
				return errors.New("gateway unreachable")
			}
			return nil
		}, &testing.PollOptions{Timeout: timeout}); err != nil {
			s.Errorf("Failed outbound check for interface %s: %s", device.PhysIfname, err)
		}

		// Translate ARC IPv4 address to string.
		arcIP := make(net.IP, 4)
		binary.LittleEndian.PutUint32(arcIP, device.Ipv4Addr)

		// This code tests inbound network to Android (ARC).
		// It fetches ARC network interfaces, and for each of the interface,
		// test if a ping to the interface is possible from the host.
		s.Logf("Pinging from host interface %s to ARC interface %s", device.Ifname, device.GuestIfname)
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			return testexec.CommandContext(ctx, "ping", "-I", device.Ifname, "-c1", "-w1", arcIP.String()).Run()
		}, &testing.PollOptions{Timeout: timeout}); err != nil {
			s.Errorf("Failed inbound check for interface %s: %s", device.PhysIfname, err)
		}
	}
}

// interfaceUp returns true if the network interface is up.
func interfaceUp(ctx context.Context, ifname string) (bool, error) {
	out, err := testexec.CommandContext(ctx, "cat", "/sys/class/net/"+ifname+"/operstate").Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) == "up", nil
}
