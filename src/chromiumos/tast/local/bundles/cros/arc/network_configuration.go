// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"net"
	"strings"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         NetworkConfiguration,
		Desc:         "Checks that the multi-networking network configuration is set up correctly",
		Contacts:     []string{"jasongustaman@google.com", "arc-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login"},
		Timeout:      4 * time.Minute,
	})
}

func NetworkConfiguration(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ARCEnabled())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	interfaceCorrespondence(ctx, s)
}

func interfaceCorrespondence(ctx context.Context, s *testing.State) {
	netIfaces, err := net.Interfaces()
	if err != nil {
		s.Fatal("Failed to get interface list: ", err)
	}

	// Differentiate between types of interfaces (device, bridge, and veth)
	type iface struct {
		veth   bool
		bridge bool
		dev    bool
		arc    bool
	}

	ifaces := make(map[string]*iface)
	for _, i := range netIfaces {
		if strings.HasPrefix(i.Name, arc.VethPrefix) {
			n := strings.TrimPrefix(i.Name, arc.VethPrefix)
			if _, ok := ifaces[n]; !ok {
				ifaces[n] = new(iface)
			}
			ifaces[n].veth = true
		} else if strings.HasPrefix(i.Name, arc.BridgePrefix) {
			n := strings.TrimPrefix(i.Name, arc.BridgePrefix)
			if _, ok := ifaces[n]; !ok {
				ifaces[n] = new(iface)
			}
			ifaces[n].bridge = true
		} else if strings.HasPrefix(i.Name, arc.VmtapPrefix) {
			continue
		} else {
			n := i.Name
			if _, ok := ifaces[n]; !ok {
				ifaces[n] = new(iface)
			}
			ifaces[n].dev = true
		}
	}

	// Checks that interface arcbr0 exists as device interface
	if i, ok := ifaces[arc.Arcbr0]; !ok || !i.dev {
		s.Error("arcbr0 interface should exist as device interface")
	}

	ifnames, err := arc.GetARCInterfaceNames(ctx)
	if err != nil {
		s.Fatal("Failed to get ARC interface list: ", err)
	}

	for _, n := range ifnames {
		ifaces[n].arc = true
	}

	// Ensure that interface arc0 exists as ARC interface
	if i, ok := ifaces[arc.Arc0]; !ok || !i.arc {
		s.Error("arc0 interface should exist as ARC interface")
	}

	// Ensure that equivalent bridge, veth, and arc interface exists
	// from device interface name
	for ifname, i := range ifaces {
		if !i.dev || strings.Compare(ifname, arc.Arcbr0) == 0 || strings.Compare(ifname, arc.Loopback) == 0 {
			continue
		}
		if !i.bridge {
			s.Errorf("Bridge interface %s should exist", arc.BridgePrefix+ifname)
		}
		if !i.arc {
			s.Errorf("ARC interface %s should exist", ifname)
		}
		if !i.veth {
			s.Errorf("veth interface %s should exist", arc.VethPrefix+ifname)
		}
	}
}
