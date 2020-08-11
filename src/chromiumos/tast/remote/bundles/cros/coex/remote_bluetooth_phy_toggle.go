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
		Func:         RemoteBluetoothPhyToggle,
		Desc:         "Verifies that WiFi does not go down on boot if Bluetooth is disabled.",
		Contacts:     []string{"billyzhao@google.com"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.coex.PhyToggle"},
		Vars:         []string{"coex.signinProfileTestExtensionManifestKey"},
	})
}

func RemoteBluetoothPhyToggle(ctx context.Context, s *testing.State) {
	req := s.RequiredVar("coex.signinProfileTestExtensionManifestKey")
	r, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect rpc: ", err)
	}
	defer r.Close(ctx)

	client := coex.NewPhyToggleClient(r.Conn)

	if _, err := client.AssertIfUp(ctx, &empty.Empty{}); err != nil {
		s.Error("Phys not up: ", err)
	}
	cred := new(coex.Credentials)
	cred.Req = req
	if _, err := client.ChangeBluetooth(ctx, cred); err != nil {
		s.Error("Could not toggle Bluetooth", err)
	}
	const pauseDuration = 5 * time.Second
	if err := testing.Sleep(ctx, pauseDuration); err != nil {
		s.Log("failed to sleep")
	}
	if err := s.DUT().Reboot(ctx); err != nil {
		s.Log("Could not reboot midtest: ", err)
	}
	if err := testing.Sleep(ctx, pauseDuration); err != nil {
		s.Log("failed to sleep")
	}

	// Cleanup. Check if we ruined anything and reboot if needed.
	if _, err := client.BringIfUp(ctx, cred); err != nil {
		s.Log("Could not bring up phys: ", err)
		if err := s.DUT().Reboot(ctx); err != nil {
			s.Log("Reboot failed: ", err)
		}
	}
}
