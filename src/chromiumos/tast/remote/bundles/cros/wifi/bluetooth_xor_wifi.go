// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

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
		Func: BluetoothXorWifi,
		Desc: "Verifies that Bluetooth and Wifi can function when the other phy is disabled",
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

func BluetoothXorWifi(ctx context.Context, s *testing.State) {
	// Clean up on exit.
	defer func(ctx context.Context) {
		d := s.DUT()
		r, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
		if err != nil {
			s.Fatal("Failed to connect rpc: ", err)
		}
		defer r.Close(ctx)
		ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
		defer cancel()
		// Enable Bluetooth device.
		btClient := network.NewBluetoothServiceClient(r.Conn)
		if _, err := btClient.SetBluetoothPoweredFast(ctx, &network.SetBluetoothPoweredFastRequest{Powered: true}); err != nil {
			s.Error("Could not enable Bluetooth: ", err)
		}
		wifiClient := network.NewWifiServiceClient(r.Conn)
		// Enable WiFi.
		if _, err := wifiClient.SetWifiEnabled(ctx, &network.SetWifiEnabledRequest{Enabled: true}); err != nil {
			s.Error("Could not enable WiFi: ", err)
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
	ctx, cancel = tf.ReserveForClose(ctx)
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

	// Validate phys can function without the other on multiple channels
	channels := [4]int{36, 149, 1, 11}
	wifiClient := network.NewWifiServiceClient(r.Conn)
	btClient := network.NewBluetoothServiceClient(r.Conn)
	for _, ch := range channels {
		if err := togglePhys(ctx, ch, btClient, tf, wifiClient, true); err != nil {
			s.Fatalf("Failed to run WiFi without Bluetooth path on channel %d: %v", ch, err)
		}
		if err := togglePhys(ctx, ch, btClient, tf, wifiClient, false); err != nil {
			s.Fatalf("Failed to run Bluetooth without WiFi path on channel %d: %v", ch, err)
		}
	}
}

func togglePhys(ctx context.Context, channel int, btClient network.BluetoothServiceClient, tf *wificell.TestFixture, wifiClient network.WifiServiceClient, enableWifiFirst bool) error {
	// Disable and Assert Wifi is down
	if err := setAssertWifi(ctx, tf, wifiClient, []int{}, false); err != nil {
		return err
	}
	// Disable Bluetooth and assert Bluetooth is down.
	if err := setAssertBluetooth(ctx, btClient, false); err != nil {
		return err
	}

	if enableWifiFirst {
		// Enable and Assert WiFi is up
		if err := setAssertWifi(ctx, tf, wifiClient, []int{channel}, true); err != nil {
			return err
		}
		// Enable and Assert Bluetooth is up.
		if err := setAssertBluetooth(ctx, btClient, true); err != nil {
			return err
		}
	} else {
		// Enable and Assert Bluetooth is up.
		if err := setAssertBluetooth(ctx, btClient, true); err != nil {
			return err
		}
		// Enable and Assert WiFi is up
		if err := setAssertWifi(ctx, tf, wifiClient, []int{channel}, true); err != nil {
			return err
		}
	}
	return nil
}

func setAssertBluetooth(ctx context.Context, btClient network.BluetoothServiceClient, enabled bool) error {
	if enabled {
		// Enable Bluetooth and assert Bluetooth is up.
		if _, err := btClient.SetBluetoothPoweredFast(ctx, &network.SetBluetoothPoweredFastRequest{Powered: true}); err != nil {
			return errors.Wrap(err, "could not enable Bluetooth")
		}
		// Validate Bluetooth adapter functionality by executing a discovery scan.
		if _, err := btClient.ValidateBluetoothFunctional(ctx, &empty.Empty{}); err != nil {
			return errors.Wrap(err, "could not validate Bluetooth status")
		}
	} else {
		// Disable Bluetooth and assert Bluetooth is down.
		if _, err := btClient.SetBluetoothPoweredFast(ctx, &network.SetBluetoothPoweredFastRequest{Powered: false}); err != nil {
			return errors.Wrap(err, "could not disable Bluetooth")
		}
	}
	return nil
}

func setAssertWifi(ctx context.Context, tf *wificell.TestFixture, wifiClient network.WifiServiceClient, channels []int, enabled bool) error {
	if enabled {
		// Enable WiFi.
		if _, err := wifiClient.SetWifiEnabled(ctx, &network.SetWifiEnabledRequest{Enabled: true}); err != nil {
			return errors.Wrap(err, "could not enable WiFi")
		}
		// Assert WiFi is functional on channel.
		if len(channels) != 1 {
			return errors.Errorf("unexpected number of channels provided, got %d", len(channels))
		}
		if err := wifiutil.AssertWifiEnabledOnChannel(ctx, tf, channels[0]); err != nil {
			return errors.Wrap(err, "Wifi not functioning")
		}
	} else {
		// Disable WiFi.
		if _, err := wifiClient.SetWifiEnabled(ctx, &network.SetWifiEnabledRequest{Enabled: false}); err != nil {
			return errors.Wrap(err, "could not disable WiFi")
		}
		// Assert WiFi is down.
		if response, err := wifiClient.GetWifiEnabled(ctx, &empty.Empty{}); err != nil {
			return errors.Wrap(err, "could not get WiFi status")
		} else if response.Enabled {
			return errors.Wrap(err, "Wifi is on, expected to be off")
		}
	}
	return nil
}
