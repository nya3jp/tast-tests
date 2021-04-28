// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RemoteIwlwifiPCIRescan,
		Desc: "Verifies that the WiFi interface will recover if removed when the device has iwlwifi_rescan",
		Contacts: []string{
			"yenlinlai@google.com",            // Test author
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"iwlwifi_rescan"},
		ServiceDeps:  []string{"tast.cros.wifi.IwlwifiPCIRescan", "tast.cros.network.WifiService"},
	})
}

func RemoteIwlwifiPCIRescan(ctx context.Context, s *testing.State) {
	r, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect rpc: ", err)
	}
	defer r.Close(ctx)

	client := wifi.NewIwlwifiPCIRescanClient(r.Conn)

	if _, err := client.RemoveIfaceAndWaitForRecovery(ctx, &empty.Empty{}); err != nil {
		s.Error("Test failed with reason: ", err)
	}

	wifiClient := wifi.NewShillServiceClient(r.Conn)
	// Cleanup. Check if we ruined anything and reboot if needed.
	if _, err := wifiClient.HealthCheck(ctx, &empty.Empty{}); err != nil {
		s.Log("Reboot DUT as the healthy check failed: ", err)
		if err := s.DUT().Reboot(ctx); err != nil {
			s.Log("Reboot failed: ", err)
		}
	}
}
