// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"chromiumos/tast/common/wifi/security/base"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell"
	ap "chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        Prefer5Ghz,
		Desc:        "Verifies that DUT can see two APs in the same network and prefer 5Ghz one",
		Contacts:    []string{"deanliao@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{"tast.cros.network.WifiService"},
		Vars:        []string{"router", "pcap"},
	})
}

func Prefer5Ghz(fullCtx context.Context, s *testing.State) {
	ops := []wificell.TFOption{
		wificell.TFCapture(true),
	}
	if router, _ := s.Var("router"); router != "" {
		ops = append(ops, wificell.TFRouter(router))
	}
	if pcap, _ := s.Var("pcap"); pcap != "" {
		ops = append(ops, wificell.TFPcap(pcap))
	}
	// As we are not in precondition, we have fullCtx as both method context and
	// daemon context.
	tf, err := wificell.NewTestFixture(fullCtx, fullCtx, s.DUT(), s.RPCHint(), ops...)
	if err != nil {
		s.Fatal("Failed to set up test fixture: ", err)
	}
	defer func() {
		if err := tf.Close(fullCtx); err != nil {
			s.Log("Failed to tear down test fixture: ", err)
		}
	}()

	ctx, cancel := tf.ReserveForClose(fullCtx)
	defer cancel()

	// Set up a 2.4GHz AP.
	channel2g := 1
	freq2g, err := ap.ChannelToFrequency(channel2g)
	if err != nil {
		s.Fatalf("Failed to look up frequency for channel %d: %s", channel2g, err)
	}
	ap2g, err := tf.ConfigureAP(
		ctx, []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(channel2g), ap.HTCaps(ap.HTCapHT20)}, nil)
	if err != nil {
		s.Fatal("Failed to configure AP: ", err)
	}
	ssid := ap2g.Config().Ssid
	defer func() {
		if err := tf.DeconfigAP(fullCtx, ap2g); err != nil {
			s.Error("Failed to deconfig AP: ", err)
		}
	}()
	ctx, _ = tf.ReserveForDeconfigAP(ctx, ap2g)

	// Set up a 5GHz AP.
	channel5g := 48
	freq5g, err := ap.ChannelToFrequency(channel5g)
	if err != nil {
		s.Fatalf("Failed to look up frequency for channel %d: %s", channel5g, err)
	}
	ap5g, err := tf.ConfigureAP(
		ctx, []ap.Option{ap.Mode(ap.Mode80211nPure), ap.Channel(channel5g), ap.SSID(ssid), ap.HTCaps(ap.HTCapHT20)}, nil)
	if err != nil {
		s.Fatal("Failed to configure AP: ", err)
	}
	defer func() {
		if err := tf.DeconfigAP(fullCtx, ap5g); err != nil {
			s.Error("Failed to deconfig AP: ", err)
		}
	}()
	ctx, _ = tf.ReserveForDeconfigAP(ctx, ap5g)
	s.Log("AP setup done")

	// Check SSID on both 2.4GHz and 5GHz channels.
	req := &network.ExpectWifiFrequenciesRequest{
		Ssid:        ssid,
		Frequencies: []uint32{uint32(freq2g), uint32(freq5g)},
	}
	_, err = tf.WifiClient().ExpectWifiFrequencies(ctx, req)
	if err != nil {
		s.Fatal("Failed to expect shill service: ", err)
	}
	s.Log("Verified that the DUT sees the SSID on both 2.4GHz and 5GHz channel")

	s.Log("Asserting the connection")
	if err := tf.ConnectWifi(ctx, ssid, false, &base.Config{}); err != nil {
		s.Fatal("Failed to connect to WiFi: ", err)
	}
	defer func() {
		if err := tf.DisconnectWifi(fullCtx); err != nil {
			s.Error("Failed to disconnect WiFi: ", err)
		}
		req := &network.DeleteEntriesForSSIDRequest{Ssid: ssid}
		if _, err := tf.WifiClient().DeleteEntriesForSSID(fullCtx, req); err != nil {
			s.Errorf("Failed to remove entries for ssid=%s: %v", ssid, err)
		}
	}()

	freqSignal, err := wifiSignal(ctx, tf, ssid)
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
	s.Log("Verified that the DUT is using 5GHz band")

	s.Log("Tearing down")
}

// wifiSignal returns a frequency-signal mapping of the given SSID.
func wifiSignal(ctx context.Context, tf *wificell.TestFixture, ssid string) (map[int]float64, error) {
	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the DUT's WiFi interface")
	}

	scanResult, err := tf.DUTIwRunner().ScanDump(ctx, iface)
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
		ret = append(ret, fmt.Sprintf("Freq: %d GHz  Signal: %.2f dBm", f, freqSignal[f]))
	}
	return strings.Join(ret, " / ")
}
