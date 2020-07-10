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
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        PowerSave,
		Desc:        "Test that we can enter and exit powersave mode without issues",
		Contacts:    []string{"arowa@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_func"},
		ServiceDeps: []string{"tast.cros.network.WifiService"},
		Vars:        []string{"router"},
	})
}

const dtimPeriod = 5

func PowerSave(fullCtx context.Context, s *testing.State) {
	router, _ := s.Var("router")
	tf, err := wificell.NewTestFixture(fullCtx, fullCtx, s.DUT(), s.RPCHint(), wificell.TFRouter(router))
	if err != nil {
		s.Fatal("Failed to set up test fixture: ", err)
	}
	defer func() {
		if err := tf.Close(fullCtx); err != nil {
			s.Error("Failed to tear down test fixture: ", err)
		}
	}()

	tfCtx, cancel := tf.ReserveForClose(fullCtx)
	defer cancel()

	ap, err := tf.ConfigureAP(tfCtx, []hostapd.Option{
		hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(48),
		hostapd.HTCaps(hostapd.HTCapHT20), hostapd.DTIMPeriod(dtimPeriod)}, nil)
	if err != nil {
		s.Fatal("Failed to configure the AP: ", err)
	}
	defer func() {
		if err := tf.DeconfigAP(tfCtx, ap); err != nil {
			s.Error("Failed to deconfig the AP: ", err)
		}
	}()
	psCtx, cancel := tf.ReserveForDeconfigAP(tfCtx, ap)
	defer cancel()

	iface, err := tf.ClientInterface(psCtx)
	if err != nil {
		s.Fatal("DUT: failed to get the client interface: ", err)
	}

	iwr := remoteiw.NewRemoteRunner(s.DUT().Conn())

	psMode, err := iwr.PowersaveMode(psCtx, iface)
	if err != nil {
		s.Fatal("DUT: failed to get the powersave mode of the WiFi interface: ", err)
	}

	defer func() {
		if err := iwr.SetPowersaveMode(psCtx, iface, psMode); err != nil {
			s.Errorf("DUT: failed to set the powersave mode %t: %v", psMode, err)
		}
	}()

	ctx, cancel := ctxutil.Shorten(psCtx, 2*time.Second)
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
	defer func() {
		if err := tf.DisconnectWifi(ctx); err != nil {
			s.Fatal("DUT: failed to disconnect WiFi: ", err)
		}
		if _, err := tf.WifiClient().DeleteEntriesForSSID(ctx, &network.DeleteEntriesForSSIDRequest{Ssid: []byte(ap.Config().SSID)}); err != nil {
			s.Errorf("Failed to remove entries for ssid=%s, err: %v", ap.Config().SSID, err)
		}
	}()

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
