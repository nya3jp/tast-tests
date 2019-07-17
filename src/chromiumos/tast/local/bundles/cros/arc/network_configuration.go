// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"net"
	"strings"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         NetworkConfiguration,
		Desc:         "Checks that the multi-networking network configuration is set up correctly",
		Contacts:     []string{"jasongustaman@google.com", "arc-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome"},
		Pre:          arc.Booted(),
		Timeout:      4 * time.Minute,
	})
}

func NetworkConfiguration(ctx context.Context, s *testing.State) {
	// Get host network interfaces.
	h, err := net.Interfaces()
	if err != nil {
		s.Fatal("Failed to get host interface list: ", err)
	}

	// Differentiate between types of interfaces (device, bridge, and veth) and check if the
	// corresponding interfaces exist.
	type iface struct {
		veth   bool
		bridge bool
		dev    bool
		arc    bool
	}

	ifaces := make(map[string]iface)
	for _, i := range h {
		if n := strings.TrimPrefix(i.Name, arc.VethPrefix); n != i.Name {
			ifc := ifaces[n]
			ifc.veth = true
			ifaces[n] = ifc
		} else if n := strings.TrimPrefix(i.Name, arc.BridgePrefix); n != i.Name {
			ifc := ifaces[n]
			ifc.bridge = true
			ifaces[n] = ifc
		} else if !strings.HasPrefix(i.Name, arc.VmtapPrefix) {
			ifc := ifaces[i.Name]
			ifc.dev = true
			ifaces[i.Name] = ifc
		}
	}

	// Checks that interface arcbr0 exists as device interface
	if ifc, ok := ifaces[arc.ARCBR0]; !ok || !ifc.dev {
		s.Error("arcbr0 interface should exist as device interface")
	}

	ifnames, err := arc.NetworkInterfaceNames(ctx)
	if err != nil {
		s.Fatal("Failed to get ARC interface list: ", err)
	}

	for _, n := range ifnames {
		ifc := ifaces[n]
		ifc.arc = true
		ifaces[n] = ifc
	}

	// Ensure that interface arc0 exists as ARC interface
	if ifc, ok := ifaces[arc.ARC0]; !ok || !ifc.arc {
		s.Error("arc0 interface should exist as ARC interface")
	}

	// Ensure that equivalent bridge, veth, and arc interface exists
	// from device interface name
	for ifname, ifc := range ifaces {
		if !ifc.dev || ifname == arc.ARCBR0 || ifname == arc.Loopback {
			continue
		}
		if !ifc.bridge {
			s.Errorf("Bridge interface %s should exist", arc.BridgePrefix+ifname)
		}
		if !ifc.arc {
			s.Errorf("ARC interface %s should exist", ifname)
		}
		if !ifc.veth {
			s.Errorf("veth interface %s should exist", arc.VethPrefix+ifname)
		}
	}
}
