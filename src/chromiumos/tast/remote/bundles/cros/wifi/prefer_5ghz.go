// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/dutcfg"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Prefer5Ghz,
		Desc: "Verifies that DUT can see two APs in the same network and prefer 5Ghz one",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_func"},
		ServiceDeps: []string{wificell.TFServiceName},
		Pre:         wificell.TestFixturePreWithCapture(),
		Vars:        []string{"router", "pcap"},
	})
}

func Prefer5Ghz(ctx context.Context, s *testing.State) {
	tf := s.PreValue().(*wificell.TestFixture)
	defer func(ctx context.Context) {
		if err := tf.CollectLogs(ctx); err != nil {
			s.Log("Error collecting logs, err: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForCollectLogs(ctx)
	defer cancel()

	// Configure an AP on the specific channel with given SSID.
	// It returns a shorten ctx, the channel's mapping frequency, a callback to deconfigure the AP run
	// with the input ctx, and an error object.
	// Note that it directly used s and tf from the outer scope.
	configureAP := func(ctx context.Context, ssid string, channel int) (context.Context, int, func(), error) {
		freq, err := hostapd.ChannelToFrequency(channel)
		if err != nil {
			return ctx, 0, nil, err
		}
		s.Logf("Setting up the AP on freq %d", freq)
		options := []hostapd.Option{hostapd.Mode(hostapd.Mode80211nPure), hostapd.Channel(channel), hostapd.HTCaps(hostapd.HTCapHT20), hostapd.SSID(ssid)}
		ap, err := tf.ConfigureAP(ctx, options, nil)
		if err != nil {
			return ctx, freq, nil, err
		}
		sCtx, cancel := tf.ReserveForDeconfigAP(ctx, ap)
		deferFunc := func() {
			cancel()
			s.Logf("Deconfiguring the AP on freq %d", freq)
			if err := tf.DeconfigAP(ctx, ap); err != nil {
				s.Error("Failed to deconfig AP: ", err)
			}
		}
		return sCtx, freq, deferFunc, nil
	}

	ssid := hostapd.RandomSSID("TAST_TEST_")
	const (
		channel2g = 1
		channel5g = 48
	)
	ctx, freq2g, deconfig2g, err := configureAP(ctx, ssid, channel2g)
	if err != nil {
		s.Fatal("Failed to set up AP: ", err)
	}
	defer deconfig2g()

	ctx, freq5g, deconfig5g, err := configureAP(ctx, ssid, channel5g)
	if err != nil {
		s.Fatal("Failed to set up AP: ", err)
	}
	defer deconfig5g()
	s.Log("AP setup done. Expecting the DUT to see the SSID on both 2.4GHz and 5GHz channels")

	// Check SSID on both 2.4GHz and 5GHz channels.
	req := &wifi.ExpectWifiFrequenciesRequest{
		Ssid:        []byte(ssid),
		Frequencies: []uint32{uint32(freq2g), uint32(freq5g)},
	}
	if _, err := tf.WifiClient().ExpectWifiFrequencies(ctx, req); err != nil {
		s.Fatal("Failed to expect a service with two WiFi frequencies: ", err)
	}
	s.Log("Verified. Asserting the connection")
	if _, err := tf.ConnectWifi(ctx, ssid, dutcfg.ConnHidden(false)); err != nil {
		s.Fatal("Failed to connect to WiFi: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.CleanDisconnectWifi(ctx); err != nil {
			s.Error("Failed to disconnect WiFi: ", err)
		}
	}(ctx)
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()

	freqSignal, err := wifiSignal(ctx, tf, s.DUT(), ssid)
	if err != nil {
		s.Fatal("Failed to get wifi signal: ", err)
	}
	s.Log("WiFi signal: ", listSignal(freqSignal))

	service, err := tf.QueryService(ctx)
	if err != nil {
		s.Fatal("Failed to get the active WiFi service from DUT: ", err)
	}
	if service.Wifi.Frequency != uint32(freq5g) {
		s.Fatalf("Got frequency %d; want %d", service.Wifi.Frequency, freq5g)
	}
	s.Log("Verified that the DUT is using 5GHz band... Tearing down")
}

// wifiSignal returns a frequency-signal mapping of the given SSID.
func wifiSignal(ctx context.Context, tf *wificell.TestFixture, dut *dut.DUT, ssid string) (map[int]float64, error) {
	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the DUT's WiFi interface")
	}

	scanResult, err := iw.NewRemoteRunner(dut.Conn()).ScanDump(ctx, iface)
	if err != nil {
		return nil, errors.Wrap(err, "failed to perform iw scan dump")
	}
	ret := map[int]float64{}
	for _, data := range scanResult {
		if data.SSID == ssid {
			ret[data.Frequency] = data.Signal
		}

	}
	return ret, nil
}

// listSignal returns a string of frequency:signal strength pairs.
func listSignal(freqSignal map[int]float64) string {
	freqs := make([]int, 0, len(freqSignal))
	for f := range freqSignal {
		freqs = append(freqs, f)
	}
	sort.Ints(freqs)
	ret := make([]string, 0, len(freqs))
	for _, f := range freqs {
		ret = append(ret, fmt.Sprintf("Freq: %dGHz  Signal: %.2fdBm", f, freqSignal[f]))
	}
	return strings.Join(ret, " / ")
}
