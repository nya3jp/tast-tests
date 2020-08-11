// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package coex

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/coex"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RemoteWifiPhyToggle,
		Desc:         "Verifies that Bluetooth does not go down on boot if Wifi is disabled",
		Contacts:     []string{"billyzhao@google.com"},
		SoftwareDeps: []string{"chrome", "reboot"},
		ServiceDeps:  []string{"tast.cros.coex.PhyToggle"},
		Vars:         []string{"coex.signinProfileTestExtensionManifestKey"},
	})
}

func RemoteWifiPhyToggle(ctx context.Context, s *testing.State) {
	// Reboot to recover umounted partitiions.
	d := s.DUT()
	req := s.RequiredVar("coex.signinProfileTestExtensionManifestKey")
	r, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect rpc: ", err)
	}
	defer func() {
		d := s.DUT()
		req := s.RequiredVar("coex.signinProfileTestExtensionManifestKey")
		r, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
		if err != nil {
			s.Fatal("Failed to connect rpc: ", err)
		}
		defer r.Close(ctx)
		cred := new(coex.Credentials)
		cred.Req = req
		client := coex.NewPhyToggleClient(r.Conn)
		if _, err := client.BringPhysUp(ctx, cred); err != nil {
			s.Log("Could not bring up phys: ", err)
			if err := d.Reboot(ctx); err != nil {
				s.Log("Reboot failed: ", err)
			}
		}
	}()

	client := coex.NewPhyToggleClient(r.Conn)
	if _, err := client.AssertPhysUp(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Phys not up: ", err)
	}
	cred := new(coex.Credentials)
	cred.Req = req
	if _, err := client.DisableWifi(ctx, cred); err != nil {
		s.Fatal("Could not toggle Wifi: ", err)
	}
	const pauseDuration = 5 * time.Second
	if err := testing.Sleep(ctx, pauseDuration); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}
	r, err = rpc.Dial(ctx, d, s.RPCHint(), "cros")
	client = coex.NewPhyToggleClient(r.Conn)
	if _, err := client.AssertWifiDown(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Wifi was not down: ", err)
	}
	if _, err := client.AssertBluetoothUp(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Bluetooth was not up: ", err)
	}
}
