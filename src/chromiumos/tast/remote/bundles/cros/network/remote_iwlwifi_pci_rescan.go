// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RemoteIwlwifiPCIRescan,
		Desc:         "Verifies that the WiFi interface will recover if removed when the device has iwlwifi_rescan",
		Contacts:     []string{"yenlinlai@google.com", "chromeos-kernel-wifi@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"iwlwifi_rescan"},
		ServiceDeps:  []string{"tast.cros.network.IwlwifiPCIRescan"},
	})
}

func RemoteIwlwifiPCIRescan(ctx context.Context, s *testing.State) {
	r, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect rpc: ", err)
	}
	defer r.Close(ctx)

	client := network.NewIwlwifiPCIRescanClient(r.Conn)

	// Cleanup. Check if we ruined anything and reboot if needed.
	defer func() {
		_, err := client.HealthCheck(ctx, &empty.Empty{})
		if err == nil {
			// No need to reboot.
			return
		}
		s.Log("Reboot DUT as the healthy check failed: ", err)
		if err := s.DUT().Reboot(ctx); err != nil {
			s.Log("Reboot failed: ", err)
		}
	}()

	_, err = client.RemoveIfaceAndWaitRecovery(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Test failed with reason: ", err)
	}
}
