// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/remote/bundles/cros/wifi/wifiutil"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PersistenceBluetoothSansWifi,
		Desc:         "Verifies that Bluetooth remains operational when Wifi is disabled on reboot",
		Contacts:     []string{"billyzhao@google.com"},
		Attr:         []string{"group:wificell", "wificell_func", "wificell_unstable"},
		SoftwareDeps: []string{"reboot"},
		ServiceDeps:  []string{wificell.TFServiceName, "tast.cros.network.BluetoothService"},
		Vars:         []string{"router"},
	})
}

func PersistenceBluetoothSansWifi(ctx context.Context, s *testing.State) {
	// Cleanup on exit.
	defer func(ctx context.Context) {
		d := s.DUT()
		r, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
		if err != nil {
			s.Fatal("Failed to connect rpc: ", err)
		}
		defer r.Close(ctx)
		wifiClient := network.NewWifiServiceClient(r.Conn)
		btClient := network.NewBluetoothServiceClient(r.Conn)
		// Enable wifi device.
		if _, err := wifiClient.SetWifiStatus(ctx, &network.SetWifiStatusRequest{Status: true}); err != nil {
			s.Error("Could not enable Wifi through shill: ", err)
		}
		// Enable bluetooth device.
		if _, err := btClient.SetBluetoothStatus(ctx, &network.SetBluetoothStatusRequest{State: true}); err != nil {
			s.Error("Could not enable Bluetooth through ui: ", err)
		}

		// Reboot the DUT.
		if err := d.Reboot(ctx); err != nil {
			s.Log("Reboot failed: ", err)
		}
	}(ctx)
	func(ctx context.Context) {
		d := s.DUT()
		r, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
		if err != nil {
			s.Fatal("Failed to connect rpc: ", err)
		}
		defer r.Close(ctx)

		// Assert wifi is up.
		router, _ := s.Var("router")
		tf, err := wificell.NewTestFixture(ctx, ctx, d, s.RPCHint(), wificell.TFRouter(router))
		if err != nil {
			s.Fatal("Failed to set up test fixture: ", err)
		}
		defer func(ctx context.Context) {
			if err := tf.Close(ctx); err != nil {
				s.Error("Failed to properly take down test fixture: ", err)
			}
			ctx, cancel := tf.ReserveForClose(ctx)
			defer cancel()
		}(ctx)

		if err := wifiutil.AssertWifiEnabled(ctx, tf); err != nil {
			s.Fatal("Wifi not functioning: ", err)
		}

		// Assert bluetooth is up.
		btClient := network.NewBluetoothServiceClient(r.Conn)
		if response, err := btClient.GetBluetoothStatus(ctx, &empty.Empty{}); err != nil {
			s.Fatal("Could not get bluetooth status: ", err)
		} else if !response.Status {
			s.Fatal("Bluetooth is off, expected to be on ")
		}

		// Disable wifi.
		wifiClient := network.NewWifiServiceClient(r.Conn)
		if _, err := wifiClient.SetWifiStatus(ctx, &network.SetWifiStatusRequest{Status: false}); err != nil {
			s.Fatal("Could not disable Wifi: ", err)
		}

		// Assert wifi is down.
		if response, err := wifiClient.GetWifiStatus(ctx, &empty.Empty{}); err != nil {
			s.Fatal("Could not disable Wifi: ", err)
		} else if response.Status {
			s.Fatal("Wifi is on, expected to be off ")
		}

	}(ctx)

	// Reboot the DUT.
	if err := s.DUT().Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}

	// Reinitialize some variables.
	d := s.DUT()
	r, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect rpc: ", err)
	}
	defer r.Close(ctx)

	// Assert wifi is down.

	wifiClient := network.NewWifiServiceClient(r.Conn)
	if response, err := wifiClient.GetWifiStatus(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Could not disable Wifi: ", err)
	} else if response.Status {
		s.Fatal("Wifi is on, expected to be off ")
	}

	// Assert bluetooth is up.
	btClient := network.NewBluetoothServiceClient(r.Conn)
	if response, err := btClient.GetBluetoothStatus(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Could not get bluetooth status: ", err)
	} else if !response.Status {
		s.Fatal("Bluetooth is off, expected to be on ")
	}
}
