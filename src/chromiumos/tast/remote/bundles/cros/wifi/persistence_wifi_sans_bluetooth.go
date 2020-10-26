// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/bundles/cros/wifi/wifiutil"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PersistenceWifiSansBluetooth,
		Desc:         "Verifies that WiFi remains operational when Bluetooth is disabled on reboot",
		Contacts:     []string{"billyzhao@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:         []string{"group:wificell", "wificell_func", "wificell_unstable"},
		SoftwareDeps: []string{"chrome", "reboot"},
		ServiceDeps:  []string{wificell.TFServiceName, "tast.cros.network.BluetoothService"},
		Vars:         []string{"router", "wifi.signinProfileTestExtensionManifestKey"},
	})
}

func PersistenceWifiSansBluetooth(ctx context.Context, s *testing.State) {
	// Cleanup on exit.
	defer func(ctx context.Context) {
		d := s.DUT()
		req := s.RequiredVar("wifi.signinProfileTestExtensionManifestKey")
		r, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
		if err != nil {
			s.Fatal("Failed to connect rpc: ", err)
		}
		defer r.Close(ctx)
		// Enable Bluetooth device.
		btClient := network.NewBluetoothServiceClient(r.Conn)
		if _, err := btClient.SetBluetoothPowered(ctx, &network.SetBluetoothPoweredRequest{Powered: true, Credentials: req}); err != nil {
			s.Error("Could not enable Bluetooth through bluetoothPrivate: ", err)
		}
	}(ctx)
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	func(ctx context.Context) {
		d := s.DUT()
		req := s.RequiredVar("wifi.signinProfileTestExtensionManifestKey")
		r, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
		if err != nil {
			s.Fatal("Failed to connect rpc: ", err)
		}
		defer r.Close(ctx)

		// Assert WiFi is up.
		var tfOps []wificell.TFOption
		if router, ok := s.Var("router"); ok && router != "" {
			tfOps = append(tfOps, wificell.TFRouter(router))
		}
		tf, err := wificell.NewTestFixture(ctx, ctx, d, s.RPCHint(), tfOps...)
		if err != nil {
			s.Fatal("Failed to set up test fixture: ", err)
		}
		defer func(ctx context.Context) {
			if err := tf.Close(ctx); err != nil {
				s.Error("Failed to properly take down test fixture: ", err)
			}
		}(ctx)
		ctx, cancel := tf.ReserveForClose(ctx)
		defer cancel()

		if err := wifiutil.AssertWifiEnabled(ctx, tf); err != nil {
			s.Fatal("Wifi not functioning: ", err)
		}

		// Assert Bluetooth is up.
		btClient := network.NewBluetoothServiceClient(r.Conn)
		if response, err := btClient.GetBluetoothPowered(ctx, &empty.Empty{}); err != nil {
			s.Fatal("Could not get Bluetooth status: ", err)
		} else if !response.Powered {
			s.Fatal("Bluetooth is off, expected to be on ")
		}
		if _, err := btClient.ValidateBluetoothFunctional(ctx, &empty.Empty{}); err != nil {
			s.Fatal("Could not get validate Bluetooth status: ", err)
		}

		// Disable Bluetooth.
		if _, err := btClient.SetBluetoothPowered(ctx, &network.SetBluetoothPoweredRequest{Powered: false, Credentials: req}); err != nil {
			s.Fatal("Could not disable Bluetooth: ", err)
		}

		// Assert Bluetooth is down.
		if response, err := btClient.GetBluetoothPowered(ctx, &empty.Empty{}); err != nil {
			s.Fatal("Could not get Bluetooth status: ", err)
		} else if response.Powered {
			s.Fatal("Bluetooth is on, expected to be off ")
		}

	}(ctx)

	// Reboot the DUT.
	if err := s.DUT().Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}

	// Reinitialize gRPC connection with DUT after reboot as the current session is now stale.
	d := s.DUT()
	r, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect rpc: ", err)
	}
	defer r.Close(ctx)

	// Assert Bluetooth is down.
	btClient := network.NewBluetoothServiceClient(r.Conn)
	if response, err := btClient.GetBluetoothPowered(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Could not get Bluetooth status: ", err)
	} else if response.Powered {
		s.Fatal("Bluetooth is on, expected to be off ")
	}

	// Assert WiFi is up.
	wifiClient := network.NewWifiServiceClient(r.Conn)
	if response, err := wifiClient.GetWifiEnabled(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Could not get WiFi status: ", err)
	} else if !response.Enabled {
		s.Fatal("Wifi is off, expected to be on")
	}

}
