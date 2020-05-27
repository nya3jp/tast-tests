// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"chromiumos/tast/common/network/iw"
	"chromiumos/tast/errors"
	remoteiw "chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        PowerSave,
		Desc:        "Test that we can enter and exit powersave mode without issues",
		Contacts:    []string{"arowa@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{"tast.cros.network.WifiService"},
		Vars:        []string{"router"},
	})
}

func PowerSave(fullCtx context.Context, s *testing.State) {
	router, _ := s.Var("router")
	tf, err := wificell.NewTestFixture(fullCtx, fullCtx, s.DUT(), s.RPCHint(), wificell.TFRouter(router))
	if err != nil {
		s.Fatal("Failed to set up test fixture: ", err)
	}
	defer func() {
		if err := tf.Close(fullCtx); err != nil {
			s.Log("Failed to tear down test fixture: ", err)
		}
	}()

	tfCtx, cancel := tf.ReserveForClose(fullCtx)
	defer cancel()

	ap, err := tf.ConfigureAP(tfCtx, []hostapd.Option{
		hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(48),
		hostapd.HTCaps(hostapd.HTCapHT20), hostapd.DTIMPeriod(5)}, nil)
	if err != nil {
		s.Fatal("Failed to configure the AP: ", err)
	}
	defer func() {
		if err := tf.DeconfigAP(tfCtx, ap); err != nil {
			s.Error("Failed to deconfig the AP: ", err)
		}
	}()
	ctx, cancel := tf.ReserveForDeconfigAP(tfCtx, ap)
	defer cancel()
	s.Log("AP setup done")

	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("DUT: failed to get the client interface: ", err)
	}

	iwr := remoteiw.NewRunner(s.DUT().Conn())

	psMode, err := iwr.PowersaveMode(ctx, iface)
	if err != nil {
		s.Fatal("DUT: failed to get the powersave mode of the WiFi interface: ", err)
	}
	defer func() {
		if err := iwr.SetPowersaveMode(ctx, iface, psMode); err != nil {
			s.Fatalf("DUT: failed to set the powersave mode %t: %v", psMode, err)
		}
	}()

	if psMode {
		if err := iwr.SetPowersaveMode(ctx, iface, false); err != nil {
			s.Fatal("DUT: failed to set the powersave mode OFF: ", err)
		}

		if err := checkPowersave(ctx, iwr, iface, false); err != nil {
			s.Fatal("DUT: failed checking the powersave mode: ", err)
		}
	}

	if err := iwr.SetPowersaveMode(ctx, iface, true); err != nil {
		s.Fatal("DUT: failed to set the powersave mode ON: ", err)
	}

	if err := checkPowersave(ctx, iwr, iface, true); err != nil {
		s.Fatal("DUT: failed checking the powersave mode: ", err)
	}

	func() {
		if _, err := tf.ConnectWifiAP(ctx, ap); err != nil {
			s.Fatal("DUT: failed to connect to WiFi: ", err)
		}
		defer func() {
			if err := tf.DisconnectWifi(ctx); err != nil {
				s.Fatal("DUT: failed to disconnect WiFi: ", err)
			}
			if _, err := tf.WifiClient().DeleteEntriesForSSID(ctx, &network.DeleteEntriesForSSIDRequest{Ssid: []byte(ap.Config().Ssid)}); err != nil {
				s.Errorf("Failed to remove entries for ssid=%s, err: %v", ap.Config().Ssid, err)
			}
		}()
		s.Log("Connected")

		if err := tf.PingFromDUT(ctx, ap.ServerIP().String()); err != nil {
			s.Fatal("Failed to ping from the DUT: ", err)
		}
		if err := tf.PingFromServer(ctx); err != nil {
			s.Fatal("Failed to ping from the Server: ", err)
		}
	}()

	if err := iwr.SetPowersaveMode(ctx, iface, false); err != nil {
		s.Fatal("DUT: failed to set the powersave mode OFF: ", err)
	}

	if err := checkPowersave(ctx, iwr, iface, false); err != nil {
		s.Fatal("DUT: failed checking the powersave mode: ", err)
	}

}

// checkPowersave returns error if powersave value is other than expected.
func checkPowersave(ctx context.Context, iwr *iw.Runner, intf string, expected bool) error {

	psMode, err := iwr.PowersaveMode(ctx, intf)
	if err != nil {
		return errors.Wrap(err, "failed to get the powersave mode of the WiFi interface")
	}

	if psMode != expected {
		return errors.Errorf("unexpected powersave mode: got %t, want %t ", psMode, expected)
	}

	return nil
}
