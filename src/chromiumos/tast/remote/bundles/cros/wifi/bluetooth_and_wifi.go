// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"

	"github.com/golang/protobuf/ptypes/empty"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BluetoothAndWifi,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies that Bluetooth and Wifi can function when the other phy is disabled",
		Contacts: []string{
			"billyzhao@google.com",            // Test author
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:         []string{"group:wificell", "wificell_func"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{wificell.TFServiceName, "tast.cros.network.BluetoothService"},
		Vars:         []string{"router"},
	})
}

func BluetoothAndWifi(ctx context.Context, s *testing.State) {
	// Clean up on exit.
	defer func(ctx context.Context) {
		d := s.DUT()
		r, err := rpc.Dial(ctx, d, s.RPCHint())
		if err != nil {
			s.Fatal("Failed to connect rpc: ", err)
		}
		defer r.Close(ctx)
		ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
		defer cancel()
		// Enable Bluetooth device.
		btClient := network.NewBluetoothServiceClient(r.Conn)
		btClient.SetupBluetoothMojo(ctx, &empty.Empty{})
		if _, err := btClient.SetBluetoothPoweredMojo(ctx, &network.SetBluetoothPoweredMojoRequest{Powered: true}); err != nil {
			s.Error("Could not enable Bluetooth: ", err)
		}
		btClient.CleanupBluetoothMojo(ctx, &empty.Empty{})

	}(ctx)
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Initialize gRPC connection with DUT.
	d := s.DUT()
	r, err := rpc.Dial(ctx, d, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect rpc: ", err)
	}
	defer r.Close(ctx)

	s.Logf("Before serivec init")

	btClient := network.NewBluetoothServiceClient(r.Conn)
	s.Logf("after serivce init")
	btClient.SetupBluetoothMojo(ctx, &empty.Empty{})
	const numIterations = 20
	for i := 0; i < numIterations; i++ {
		s.Logf("Iteration %d of %d", i, numIterations)
		if _, err := btClient.SetBluetoothPoweredMojo(ctx, &network.SetBluetoothPoweredMojoRequest{Powered: true}); err != nil {
			s.Error("Could not enable Bluetooth: ", err)
		}
		s.Logf("after bt enable")

		if _, err := btClient.SetBluetoothPoweredMojo(ctx, &network.SetBluetoothPoweredMojoRequest{Powered: false}); err != nil {
			s.Error("Could not disable Bluetooth: ", err)
		}
		s.Logf("after bt disable")
	}
	s.Logf("iteration done")
	btClient.CleanupBluetoothMojo(ctx, &empty.Empty{})
	s.Logf("cleanup done")
}
