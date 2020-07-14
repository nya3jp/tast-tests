// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"net"
	"time"

	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/wpasupplicant"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     POCLocal,
		Desc:     "POC anything",
		Contacts: []string{"yenlinlai@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

func POCLocal(ctx context.Context, s *testing.State) {
	m, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to create shill manager: ", err)
	}
	ifaceName, err := shill.WifiInterface(ctx, m, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to get WiFi interface: ", err)
	}
	supplicant, err := wpasupplicant.NewSupplicant(ctx)
	if err != nil {
		s.Fatal("Failed to connect to wpa_supplicant: ", err)
	}
	iface, err := supplicant.GetInterface(ctx, ifaceName)
	if err != nil {
		s.Fatal("Failed to get interface object paths: ", err)
	}
	bsses, err := iface.BSSs(ctx)
	if err != nil {
		s.Fatal("Failed to get BSSs: ", err)
	}
	for _, bss := range bsses {
		// TODO: we might also have race that bss is already removed
		// between BSSs and BSSID call.
		bssid, err := bss.BSSID(ctx)
		if err != nil {
			s.Log("Failed to get BSSID for bss: ", err)
		}
		s.Logf("\t%q", net.HardwareAddr(bssid))
	}
}
