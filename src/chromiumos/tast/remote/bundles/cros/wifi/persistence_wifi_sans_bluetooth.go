// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PersistenceWifiSansBluetooth,
		Desc:         "Verifies that Wifi remains operational when Bluetooth is disabled on reboot",
		Contacts:     []string{"billyzhao@google.com"},
		Attr:         []string{"group:wificell", "wificell_func", "wificell_unstable"},
		SoftwareDeps: []string{"chrome", "reboot"},
		ServiceDeps:  []string{wificell.TFServiceName, "tast.cros.network.BluetoothService"},
		Vars:         []string{"router", "wifi.signinProfileTestExtensionManifestKey"},
	})
}

func PersistenceWifiSansBluetooth(ctx context.Context, s *testing.State) {
	d := s.DUT()
	req := s.RequiredVar("wifi.signinProfileTestExtensionManifestKey")
	r, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect rpc: ", err)
	}

	// Cleanup on exit
	defer func(ctx context.Context) {
		d := s.DUT()
		req := s.RequiredVar("wifi.signinProfileTestExtensionManifestKey")
		r, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
		if err != nil {
			s.Fatal("Failed to connect rpc: ", err)
		}
		defer r.Close(ctx)
		wifiClient := network.NewWifiServiceClient(r.Conn)
		btClient := network.NewBluetoothServiceClient(r.Conn)
		// Enable wifi
		wifiClient.SetWifiStatus(ctx, &network.TechnologyToggleRequest{Status: true})
		// Enable bluetooth
		btClient.SetBluetoothStatus(ctx, &network.BluetoothStatusRequest{State: true, Credentials: req})

		// Reboot
		if err := d.Reboot(ctx); err != nil {
			s.Log("Reboot failed: ", err)
		}
	}(ctx)

	// Test wifi is up
	assertWifiEnabled := func(ctx context.Context) {
		router, _ := s.Var("router")
		tf, err := wificell.NewTestFixture(ctx, ctx, d, s.RPCHint(), wificell.TFRouter(router))
		if err != nil {
			s.Fatal("Failed to set up test fixture: ", err)
		}
		defer func(ctx context.Context) {
			if err := tf.Close(ctx); err != nil {
				s.Log("Failed to tear down test fixture: ", err)
			}
		}(ctx)
		ctx, cancel := tf.ReserveForClose(ctx)
		defer cancel()
		ap, err := tf.DefaultOpenNetworkAP(ctx)
		if err != nil {
			s.Fatal("Failed to configure the AP: ", err)
		}
		defer func(ctx context.Context) {
			if err := tf.DeconfigAP(ctx, ap); err != nil {
				s.Error("Failed to deconfig the AP: ", err)
			}
		}(ctx)
		ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap)
		defer cancel()
		s.Log("AP setup done")

		_, err = tf.ConnectWifiAP(ctx, ap)
		if err != nil {
			s.Fatal("Failed to connect to WiFi: ", err)
		}
		defer func(ctx context.Context) {
			if err := tf.DisconnectWifi(ctx); err != nil {
				s.Error("Failed to disconnect WiFi: ", err)
			}
			req := &network.DeleteEntriesForSSIDRequest{Ssid: []byte(ap.Config().SSID)}
			if _, err := tf.WifiClient().DeleteEntriesForSSID(ctx, req); err != nil {
				s.Errorf("Failed to remove entries for ssid=%s: %v", ap.Config().SSID, err)
			}
		}(ctx)
		ctx, cancel = tf.ReserveForDisconnect(ctx)
		defer cancel()
		s.Log("Connected")

		if err := tf.PingFromDUT(ctx, ap.ServerIP().String()); err != nil {
			s.Fatal("Failed to ping from the DUT: ", err)
		}

		_, err = tf.ClientInterface(ctx)
		if err != nil {
			s.Fatal("Failed to get interface from the DUT: ", err)
		}
	}
	assertWifiEnabled(ctx)

	// Assert bluetooth is up
	btClient := network.NewBluetoothServiceClient(r.Conn)
	if response, err := btClient.GetBluetoothStatus(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Could not get bluetooth status: ", err)
	} else if !response.Status {
		s.Fatal("Bluetooth is off, expected to be on: ", err)
	}

	// Disable bluetooth
	if _, err := btClient.SetBluetoothStatus(ctx, &network.BluetoothStatusRequest{State: false, Credentials: req}); err != nil {
		s.Fatal("Could not Disable Bluetooth: ", err)
	}

	// Assert bluetooth is down
	if response, err := btClient.GetBluetoothStatus(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Could not get bluetooth status: ", err)
	} else if response.Status {
		s.Fatal("Bluetooth is on, expected to be off: ", err)
	}

	// Reboot DUT
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}

	// Assert bluetooth is down
	r, err = rpc.Dial(ctx, d, s.RPCHint(), "cros")
	btClient = network.NewBluetoothServiceClient(r.Conn)
	if response, err := btClient.GetBluetoothStatus(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Could not get bluetooth status: ", err)
	} else if response.Status {
		s.Fatal("Bluetooth is on, expected to be off: ", err)
	}
	// Assert wifi is up
	assertWifiEnabled(ctx)
}
