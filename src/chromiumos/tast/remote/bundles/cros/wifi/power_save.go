// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/common/network/iw"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	remoteiw "chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PowerSave,
		Desc: "Test that we can enter and exit powersave mode without issues",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_func"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixt",
	})
}

func PowerSave(ctx context.Context, s *testing.State) {
	const dtimPeriod = 5

	tf := s.FixtValue().(*wificell.TestFixture)

	ap, err := tf.ConfigureAP(ctx, []hostapd.Option{
		hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(48),
		hostapd.HTCaps(hostapd.HTCapHT20), hostapd.DTIMPeriod(dtimPeriod)}, nil)
	if err != nil {
		s.Fatal("Failed to configure the AP: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, ap); err != nil {
			s.Error("Failed to deconfig the AP: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap)
	defer cancel()

	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("DUT: failed to get the client interface: ", err)
	}

	iwr := remoteiw.NewRemoteRunner(s.DUT().Conn())

	psMode, err := iwr.PowersaveMode(ctx, iface)
	if err != nil {
		s.Fatal("DUT: failed to get the powersave mode of the WiFi interface: ", err)
	}

	defer func(ctx context.Context) {
		if err := iwr.SetPowersaveMode(ctx, iface, psMode); err != nil {
			s.Errorf("DUT: failed to set the powersave mode %t: %v", psMode, err)
		}
	}(ctx)
	ctx, cancel = ctxutil.Shorten(ctx, 2*time.Second)
	defer cancel()

	if psMode {
		if err := setPowersaveMode(ctx, iwr, iface, false); err != nil {
			s.Fatal("DUT: failed to set the powersave mode OFF: ", err)
		}
	}

	// TODO(b:158222331) Check if it is important to test switching
	// the powersave ON before connecting to the AP.

	if _, err := tf.ConnectWifiAP(ctx, ap); err != nil {
		s.Fatal("DUT: failed to connect to WiFi: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.CleanDisconnectWifi(ctx); err != nil {
			s.Error("DUT: failed to disconnect WiFi: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()

	// TODO(b:158222331) Make sure there is no background
	// service that also toggles the powersave mode.
	if err := setPowersaveMode(ctx, iwr, iface, true); err != nil {
		s.Fatal("DUT: failed to set the powersave mode ON: ", err)
	}

	dtimVal, err := iwr.LinkValue(ctx, iface, iw.LinkKeyDtimPeriod)
	if err != nil {
		s.Fatal("DUT: failed to get the DTIM value: ", err)
	}

	dtimValInt, err := strconv.Atoi(dtimVal)
	if err != nil {
		s.Fatal("Failed to convert the DTIM string to integer: ", err)
	}

	if dtimValInt != dtimPeriod {
		s.Fatalf("Unexpected DTIM period: got %d; want %d ", dtimValInt, dtimPeriod)
	}

	if err := tf.PingFromDUT(ctx, ap.ServerIP().String()); err != nil {
		s.Fatal("Failed to ping from the DUT: ", err)
	}

	if err := tf.PingFromServer(ctx); err != nil {
		s.Fatal("Failed to ping from the Server: ", err)
	}

	if err := setPowersaveMode(ctx, iwr, iface, false); err != nil {
		s.Fatal("DUT: failed to set the powersave mode OFF: ", err)
	}

	if err := tf.PingFromDUT(ctx, ap.ServerIP().String()); err != nil {
		s.Fatal("Failed to ping from the DUT: ", err)
	}

	if err := tf.PingFromServer(ctx); err != nil {
		s.Fatal("Failed to ping from the Server: ", err)
	}
}

// setPowersaveMode sets the powersave mode and verifies its new value.
func setPowersaveMode(ctx context.Context, iwr *iw.Runner, iface string, mode bool) error {
	if err := iwr.SetPowersaveMode(ctx, iface, mode); err != nil {
		return err
	}

	psMode, err := iwr.PowersaveMode(ctx, iface)
	if err != nil {
		return errors.Wrap(err, "failed to get the powersave mode of the WiFi interface")
	}

	if psMode != mode {
		return errors.Errorf("unexpected powersave mode: got %t; want %t", psMode, mode)
	}

	return nil
}
