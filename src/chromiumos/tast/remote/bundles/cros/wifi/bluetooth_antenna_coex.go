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
		Func:         BluetoothAntennaCoex,
		Desc:         "Verifies that Bluetooth and Wifi can function either phy is disabled",
		Contacts:     []string{"billyzhao@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:         []string{"group:wificell", "wificell_func", "wificell_unstable"},
		SoftwareDeps: []string{"chrome", "reboot"},
		ServiceDeps:  []string{wificell.TFServiceName, "tast.cros.network.BluetoothService"},
		Vars:         []string{"router", "wifi.signinProfileTestExtensionManifestKey"},
	})
}

func BluetoothAntennaCoex(ctx context.Context, s *testing.State) {
	// Cleanup on exit.
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
			s.Error("Could not enable Bluetooth: ", err)
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

		// Assert Bluetooth is up.
		btClient := network.NewBluetoothServiceClient(r.Conn)
		if response, err := btClient.GetBluetoothPowered(ctx, &network.GetBluetoothPoweredRequest{Credentials: credKey}); err != nil {
			s.Fatal("Could not get Bluetooth status: ", err)
		} else if !response.Powered {
			s.Fatal("Bluetooth is off, expected to be on ")
		}
		if _, err := btClient.ValidateBluetoothFunctional(ctx, &empty.Empty{}); err != nil {
			s.Fatal("Could not get validate Bluetooth status: ", err)
		}

		// Validate coex on multiple channels
		channels := [4]int{36, 149, 1, 11}
		wifiClient := network.NewWifiServiceClient(r.Conn)
		for _, ch := range channels {
			if err := testWifiSansBluetooth(ctx, ch, btClient, tf, wifiClient, credKey); err != nil {
				s.Fatalf("Failed to run WiFi without Bluetooth path on channel %d: %v", ch, err)
			}
			if err := testBluetoothSansWifi(ctx, ch, btClient, tf, wifiClient, credKey); err != nil {
				s.Fatalf("Failed to run Bluetooth without WiFi path on channel %d: %v", ch, err)
			}
		}
	}(ctx)

}

func testWifiSansBluetooth(ctx context.Context, channel int, btClient network.BluetoothServiceClient, tf *wificell.TestFixture, wifiClient network.WifiServiceClient, credKey string) error {
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

	// Disable Bluetooth.
	if _, err := btClient.SetBluetoothPoweredFast(ctx, &network.SetBluetoothPoweredFastRequest{Powered: false}); err != nil {
		return errors.Wrap(err, "could not disable Bluetooth")
	}

	// Assert Bluetooth is down.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if response, err := btClient.GetBluetoothPoweredFast(ctx, &empty.Empty{}); err != nil {
			return errors.Wrap(err, "could not get Bluetooth status")
		} else if response.Powered {
			return errors.New("Bluetooth is on, expected to be off")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  10 * time.Second,
		Interval: time.Second,
	}); err != nil {
		return err
	}
	// Enable WiFi.
	if _, err := wifiClient.SetWifiEnabled(ctx, &network.SetWifiEnabledRequest{Enabled: true}); err != nil {
		return errors.Wrap(err, "could not enable WiFi")
	}
	// Assert WiFi is up.
	if err := wifiutil.AssertWifiEnabledOnChannel(ctx, tf, channel); err != nil {
		return errors.Wrap(err, "Wifi not functioning")
	}
	// Enable Bluetooth.
	if _, err := btClient.SetBluetoothPoweredFast(ctx, &network.SetBluetoothPoweredFastRequest{Powered: true}); err != nil {
		return errors.Wrap(err, "could not enable Bluetooth")
	}
	// Assert Bluetooth is up.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if response, err := btClient.GetBluetoothPoweredFast(ctx, &empty.Empty{}); err != nil {
			return errors.Wrap(err, "could not get Bluetooth status")
		} else if !response.Powered {
			return errors.New("Bluetooth is off, expected to be on")
		}
		if _, err := btClient.ValidateBluetoothFunctional(ctx, &empty.Empty{}); err != nil {
			return errors.Wrap(err, "could not validate Bluetooth status")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  10 * time.Second,
		Interval: time.Second,
	}); err != nil {
		return err
	}
	return nil
}

func testBluetoothSansWifi(ctx context.Context, channel int, btClient network.BluetoothServiceClient, tf *wificell.TestFixture, wifiClient network.WifiServiceClient, credKey string) error {
	// Disable Bluetooth.
	if _, err := btClient.SetBluetoothPoweredFast(ctx, &network.SetBluetoothPoweredFastRequest{Powered: false}); err != nil {
		return errors.Wrap(err, "could not disable Bluetooth")
	}

	// Assert Bluetooth is down.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if response, err := btClient.GetBluetoothPoweredFast(ctx, &empty.Empty{}); err != nil {
			return errors.Wrap(err, "could not get Bluetooth status")
		} else if response.Powered {
			return errors.New("Bluetooth is on, expected to be off")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  10 * time.Second,
		Interval: time.Second,
	}); err != nil {
		return err
	}
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
	// Enable Bluetooth.
	if _, err := btClient.SetBluetoothPoweredFast(ctx, &network.SetBluetoothPoweredFastRequest{Powered: true}); err != nil {
		return errors.Wrap(err, "could not enable Bluetooth")
	}
	// Assert Bluetooth is up.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if response, err := btClient.GetBluetoothPoweredFast(ctx, &empty.Empty{}); err != nil {
			return errors.Wrap(err, "could not get Bluetooth status")
		} else if !response.Powered {
			return errors.New("Bluetooth is off, expected to be on")
		}
		if _, err := btClient.ValidateBluetoothFunctional(ctx, &empty.Empty{}); err != nil {
			return errors.Wrap(err, "could not validate Bluetooth status")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  10 * time.Second,
		Interval: time.Second,
	}); err != nil {
		return err
	}
	// Enable WiFi.
	if _, err := wifiClient.SetWifiEnabled(ctx, &network.SetWifiEnabledRequest{Enabled: true}); err != nil {
		return errors.Wrap(err, "could not enable WiFi")
	}
	// Assert WiFi is up.
	if err := wifiutil.AssertWifiEnabledOnChannel(ctx, tf, channel); err != nil {
		return errors.Wrap(err, "Wifi not functioning")
	}
	return nil
}
