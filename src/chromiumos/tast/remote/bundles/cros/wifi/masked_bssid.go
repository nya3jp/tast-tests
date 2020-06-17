// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"

	remoteiw "chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        MaskedBSSID,
		Desc:        "Test behavior around masked BSSIDs",
		Contacts:    []string{"arowa@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{"tast.cros.network.WifiService"},
		Vars:        []string{"router"},
	})
}

func MaskedBSSID(fullCtx context.Context, s *testing.State) {
	// Set up two APs on the same channel/bssid but with different SSIDs.
	// Check that we can see both APs in scan results.

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
	// It returns a shorten ctx, the channel's mapping frequency, a callback to deconfigure the AP and an error object.
	// Note that it directly used s and tf from the outer scope.
	configureAP := func(ctx context.Context, ssid string, channel int) (context.Context, int, func(context.Context), error) {
		freq, err := hostapd.ChannelToFrequency(channel)
		if err != nil {
			return ctx, 0, nil, err
		}
		s.Logf("Setting up the AP on freq %d", freq)
		options := []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(channel), hostapd.HTCaps(hostapd.HTCapHT20), hostapd.BSSID("00:11:22:33:44:55"), hostapd.SSID(ssid)}
		ap, err := tf.ConfigureAP(ctx, options, nil)
		if err != nil {
			return ctx, freq, nil, err
		}
		sCtx, _ := tf.ReserveForDeconfigAP(ctx, ap)
		deferFunc := func(ctx context.Context) {
			s.Logf("Deconfiguring the AP on freq %d", freq)
			if err := tf.DeconfigAP(ctx, ap); err != nil {
				s.Error("Failed to deconfig AP: ", err)
			}
		}
		return sCtx, freq, deferFunc, nil
	}

	ssid1 := hostapd.RandomSSID("TAST_TEST_1")
	ssid2 := hostapd.RandomSSID("TAST_TEST_2")
	const (
		channel1 = 1
		channel2 = 36
	)

	// Create an AP, manually specifying both the SSID and BSSID.
	ctx, freq1, deconfig1, err := configureAP(ctx, ssid1, channel1)
	if err != nil {
		s.Fatal("Failed to set up AP: ", err)
	}
	defer deconfig1(fullCtx)

	// Create a second AP that responds to probe requests with the same BSSID
	// but a different SSID. These APs together are meant to emulate
	// situations that occur with some types of APs which broadcast or
	// respond with more than one (non-empty) SSID.
	ctx, freq2, deconfig2, err := configureAP(ctx, ssid2, channel2)
	if err != nil {
		s.Fatal("Failed to set up AP: ", err)
	}
	defer deconfig2(fullCtx)

	// Background full scan, which means the scan is performed with a established connection.
	// Disable background scan mode first.
	s.Log("Disable the DUT's WiFi background scan")
	method, err := tf.WifiClient().GetBgscanMethod(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Unable to get the DUT's WiFi bgscan method: ", err)
	}
	if _, err := tf.WifiClient().SetBgscanMethod(ctx, &network.SetBgscanMethodRequest{Method: "none"}); err != nil {
		s.Fatal("Unable to stop the DUT's WiFi bgscan: ", err)
	}
	defer func() {
		s.Log("Restore the DUT's WiFi background scan to ", method.Method)
		if _, err := tf.WifiClient().SetBgscanMethod(ctx, &network.SetBgscanMethodRequest{Method: method.Method}); err != nil {
			s.Errorf("Failed to restore the DUT's bgscan method to %s: %v", method.Method, err)
		}
	}()

	// We cannot connect to this AP, since there are two separate APs that
	// respond to the same BSSID, but we can test to make sure both SSIDs
	// appears in the scan.
	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("DUT: failed to get the client interface: ", err)
	}

	res, err := remoteiw.NewRemoteRunner(s.DUT().Conn()).TimedScan(ctx, iface, []int{freq1, freq2}, nil)
	if err != nil {
		s.Fatal("TimedScan failed: ", err)
	}

	var foundSSIDs bool
	for _, ssid := range []string{ssid1, ssid2} {
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
		s.Errorf("DUT: failed to find the ssids=%s,%s in the scan", ssid1, ssid2)
	}

}
