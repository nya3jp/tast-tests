// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/wifi/wifiutil"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PersistenceWifiSansBluetooth,
		Desc: "Verifies that WiFi remains operational when Bluetooth is disabled on reboot",
		Contacts: []string{
			"billyzhao@google.com",            // Test author
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:         []string{"group:wificell", "wificell_func", "wificell_unstable"},
		SoftwareDeps: []string{"chrome", "reboot"},
		ServiceDeps:  []string{wificell.TFServiceName, "tast.cros.network.BluetoothService"},
		Vars:         []string{"router", "wifi.signinProfileTestExtensionManifestKey"},
	})
}

func PersistenceWifiSansBluetooth(ctx context.Context, s *testing.State) {
	// Clean up on exit.
	credKey := s.RequiredVar("wifi.signinProfileTestExtensionManifestKey")
	defer func(ctx context.Context) {
		d := s.DUT()
		r, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
		if err != nil {
			s.Fatal("Failed to connect rpc: ", err)
		}
		defer r.Close(ctx)
		// Enable Bluetooth device.
		btClient := network.NewBluetoothServiceClient(r.Conn)
		if _, err := btClient.SetBluetoothPowered(ctx, &network.SetBluetoothPoweredRequest{Powered: true, Credentials: credKey}); err != nil {
			s.Error("Could not enable Bluetooth through bluetoothPrivate: ", err)
		}
	}(ctx)
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Initialize TestFixture Options.
	var tfOps []wificell.TFOption
	if router, ok := s.Var("router"); ok && router != "" {
		tfOps = append(tfOps, wificell.TFRouter(router))
	}
	tfOps = append(tfOps, wificell.TFWithUI())

	func(ctx context.Context) {
		// Assert WiFi is up.
		tf, err := wificell.NewTestFixture(ctx, ctx, s.DUT(), s.RPCHint(), tfOps...)
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

		// Initialize gRPC connection with DUT.
		d := s.DUT()
		r, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
		if err != nil {
			s.Fatal("Failed to connect rpc: ", err)
		}
		defer r.Close(ctx)

		// Disable Bluetooth and assert Bluetooth is down.
		btClient := network.NewBluetoothServiceClient(r.Conn)
		if _, err := btClient.SetBluetoothPowered(ctx, &network.SetBluetoothPoweredRequest{Powered: false, Credentials: credKey}); err != nil {
			s.Fatal("Could not disable Bluetooth: ", err)
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
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if response, err := btClient.GetBluetoothPowered(ctx, &network.GetBluetoothPoweredRequest{Credentials: credKey}); err != nil {
			return errors.Wrap(err, "could not get Bluetooth status after boot")
		} else if response.Persistent {
			return testing.PollBreak(errors.Wrap(err, "Bluetooth is set to start on boot, should be off on boot"))
		} else if response.Powered {
			return errors.New("Bluetooth is on, expected to be off after boot")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  10 * time.Second,
		Interval: time.Second,
	}); err != nil {
		s.Fatal("Failed to wait for BT to be powered: ", err)
	}

	// Assert WiFi is up.
	tf, err := wificell.NewTestFixture(ctx, ctx, d, s.RPCHint(), tfOps...)
	if err != nil {
		s.Fatal("Failed to set up test fixture: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.Close(ctx); err != nil {
			s.Error("Failed to properly take down test fixture: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForClose(ctx)
	defer cancel()

	if err := wifiutil.AssertWifiEnabled(ctx, tf); err != nil {
		s.Fatal("Wifi not functioning: ", err)
	}
}
