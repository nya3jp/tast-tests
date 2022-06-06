// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"time"

	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/bluetooth"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: BluetoothRemoteTest,
		Desc: "Example remote test for bluetooth",
		Contacts: []string{
			"shijinabraham@google.com", // Test author
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.bluetooth.BluetoothMojoService"},
		Timeout:      7 * time.Minute,
	})
}

func BluetoothRemoteTest(ctx context.Context, s *testing.State) {

	// Establish RPC connection to the DUT.
	cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	// Create client instance of the SystemTimezone service.
	client := bluetooth.NewBluetoothMojoServiceClient(cl.Conn)

	// Use the TestSystemTimezone method of the SystemTimezone service
	// to check if the timezone was set correctly by the policy.
	if _, err = client.SetBluetoothState(ctx, True); err != nil {
		s.Error("Failed to set bluetotoh state : ", err)
	} else {
		s.LogF("Bluetooth state set")
	}

}
