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
		Func:         PersistenceBluetoothSansWifi,
		Desc:         "Verifies that Bluetooth remains operational when Wifi is disabled on reboot",
		Contacts:     []string{"billyzhao@google.com", "chromeos-platform-connectivity@google.com"},
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
		// Enable wifi device.
		wifiClient := network.NewWifiServiceClient(r.Conn)
		if _, err := wifiClient.SetWifiEnabled(ctx, &network.SetWifiEnabledRequest{Enabled: true}); err != nil {
			s.Error("Could not enable Wifi through shill: ", err)
		}
	}(ctx)
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	func(ctx context.Context) {
		d := s.DUT()
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

		// Assert bluetooth is up.
		btClient := network.NewBluetoothServiceClient(r.Conn)
		if response, err := btClient.GetBluetoothPowered(ctx, &empty.Empty{}); err != nil {
			s.Fatal("Could not get Bluetooth status: ", err)
		} else if !response.Powered {
			s.Fatal("Bluetooth is off, expected to be on ")
		}
		if _, err := btClient.ValidateBluetoothFunctional(ctx, &empty.Empty{}); err != nil {
			s.Fatal("Could not get validate Bluetooth status: ", err)
		}

		// Disable WiFi.
		wifiClient := network.NewWifiServiceClient(r.Conn)
		if _, err := wifiClient.SetWifiEnabled(ctx, &network.SetWifiEnabledRequest{Enabled: false}); err != nil {
			s.Fatal("Could not disable Wifi: ", err)
		}

		// Assert WiFi is down.
		if response, err := wifiClient.GetWifiEnabled(ctx, &empty.Empty{}); err != nil {
			s.Fatal("Could not get WiFi status: ", err)
		} else if response.Enabled {
			s.Fatal("Wifi is on, expected to be off ")
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

	// Assert WiFi is down.
	wifiClient := network.NewWifiServiceClient(r.Conn)
	if response, err := wifiClient.GetWifiEnabled(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Could not get WiFi status: ", err)
	} else if response.Enabled {
		s.Fatal("Wifi is on, expected to be off ")
	}

	// Assert Bluetooth is up. We need to poll a little bit here as it might
	// not yet get initialized after reboot.
	btClient := network.NewBluetoothServiceClient(r.Conn)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if response, err := btClient.GetBluetoothPowered(ctx, &empty.Empty{}); err != nil {
			return errors.Wrap(err, "could not get Bluetooth status")
		} else if !response.Powered {
			return errors.New("Bluetooth is off, expected to be on")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  10 * time.Second,
		Interval: time.Second,
	}); err != nil {
		s.Fatal("Failed to wait for BT to be powered: ", err)
	}
	if _, err := btClient.ValidateBluetoothFunctional(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Could not get validate Bluetooth status: ", err)
	}
}
