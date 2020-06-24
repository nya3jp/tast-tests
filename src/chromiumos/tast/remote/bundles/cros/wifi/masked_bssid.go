// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/network/iw"
	remoteiw "chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        MaskedBSSID,
		Desc:        "Test that two APs with the same BSSID, but with different SSIDs can both be seen in the scan results",
		Contacts:    []string{"arowa@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{"tast.cros.network.WifiService"},
		Vars:        []string{"router"},
	})
}

func MaskedBSSID(fullCtx context.Context, s *testing.State) {
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

	ctx, cancel := tf.ReserveForClose(fullCtx)
	defer cancel()

	// Configure an AP on the specific channel with given SSID.
	// It returns a shorten ctx, an APIface object, a callback to deconfigure the AP and an error object.
	// Note that it directly used s and tf from the outer scope.
	configureAP := func(ctx context.Context, ssid string, channel int) (context.Context, *wificell.APIface, func(context.Context), error) {
		s.Logf("Setting up the AP on channel %d", channel)
		options := []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(channel), hostapd.HTCaps(hostapd.HTCapHT20), hostapd.BSSID("00:11:22:33:44:55"), hostapd.SSID(ssid)}
		ap, err := tf.ConfigureAP(ctx, options, nil)
		if err != nil {
			return ctx, nil, nil, err
		}
		sCtx, cancel := tf.ReserveForDeconfigAP(ctx, ap)
		deferFunc := func(ctx context.Context) {
			s.Logf("Deconfiguring the AP on channel %d", channel)
			if err := tf.DeconfigAP(ctx, ap); err != nil {
				s.Error("Failed to deconfig AP: ", err)
			}
			cancel()
		}
		return sCtx, ap, deferFunc, nil
	}

	// Create an AP, manually specifying both the SSID and BSSID.
	// Then create a second AP that responds to probe requests with
	// the same BSSID but a different SSID. These APs together are
	// meant to emulate situations that occur with some types of APs
	// which broadcast or respond with more than one (non-empty) SSID.
	channels := []int{1, 36}
	var ssids []string
	var freqs []int
	var aps []*wificell.APIface
	for i, ch := range channels {
		if freq, err := hostapd.ChannelToFrequency(ch); err != nil {
			s.Fatalf("Invalid channel %d: %v", ch, err)
		} else {
			freqs = append(freqs, freq)
		}

		ssid := hostapd.RandomSSID(fmt.Sprintf("TAST_TEST_%d_", i))
		sCtx, ap, deconfig, err := configureAP(ctx, ssid, ch)
		if err != nil {
			s.Fatal("Failed to set up AP: ", err)
		}
		defer deconfig(fullCtx)
		ssids = append(ssids, ssid)
		aps = append(aps, ap)
		ctx = sCtx
	}

	// Test to make sure both SSIDs appears in the scan.
	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("DUT: ", err)
	}

	var res *iw.TimedScanData
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		res, err = remoteiw.NewRemoteRunner(s.DUT().Conn()).TimedScan(ctx, iface, freqs, ssids)
		if err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 500 * time.Millisecond}); err != nil {
		s.Fatal("DUT: failed to run TimedScan: ", err)
	}

	var foundSSIDs bool
	for _, ssid := range ssids {
		foundSSIDs = false
		for _, data := range res.BSSList {
			if ssid == data.SSID {
				foundSSIDs = true
				break
			}
		}
		if !foundSSIDs {
			break
		}
	}

	if !foundSSIDs {
		s.Errorf("DUT: failed to find the ssids=%v in the scan", ssids)
	}

	for _, ap := range aps {
		if _, err := tf.ConnectWifiAP(ctx, ap); err != nil {
			s.Errorf("Failed to connect to WiFi SSID %s: %v", ap.Config().SSID, err)
		}
		if err := tf.PingFromDUT(ctx, ap.ServerIP().String()); err != nil {
			s.Error("Failed to ping from the Server: ", err)
		}
		if err := tf.DisconnectWifi(ctx); err != nil {
			s.Error("Failed to disconnect WiFi: ", err)
		}
	}
}
