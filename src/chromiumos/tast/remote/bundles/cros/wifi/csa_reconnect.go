// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/ctxutil"
	remoteiw "chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        CSAReconnect,
		Desc:        "Verifies that DUT will connect to the new channel after the AP sends a Spectrum Management action frame with a Channel Move element",
		Contacts:    []string{"yenlinlai@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{wificell.TFServiceName},
		Pre:         wificell.TestFixturePre(),
		Vars:        []string{"router", "pcap"},
	})
}

func CSAReconnect(ctx context.Context, s *testing.State) {
	// Note: Not all clients support CSA, but they generally should at least try
	// to disconnect from the AP which is what the test expects to see.

	tf := s.PreValue().(*wificell.TestFixture)
	defer func(ctx context.Context) {
		if err := tf.CollectLogs(ctx); err != nil {
			s.Log("Error collecting logs, err: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForCollectLogs(ctx)
	defer cancel()

	// TODO(b/154879577): Currently the action frames sent by FrameSender
	// are not buffered for DTIM so if the DUT is in powersave mode, it
	// cannot receive the action frame and the test will fail.
	// Turn off powersave mode to replicate the behavior of Autotest in
	// this test for now.
	iwr := remoteiw.NewRemoteRunner(s.DUT().Conn())
	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("Failed to get the client interface: ", err)
	}
	psMode, err := iwr.PowersaveMode(ctx, iface)
	if err != nil {
		s.Fatal("Failed to get the powersave mode: ", err)
	}
	if psMode {
		defer func(ctx context.Context) {
			s.Logf("Restoring power save mode to %t", psMode)
			if err := iwr.SetPowersaveMode(ctx, iface, psMode); err != nil {
				s.Errorf("Failed to restore powersave mode to %t: %v", psMode, err)
			}
		}(ctx)
		ctx, cancel = ctxutil.Shorten(ctx, time.Second)
		defer cancel()

		s.Log("Disabling power save in the test")
		if err := iwr.SetPowersaveMode(ctx, iface, false); err != nil {
			s.Fatal("Failed to turn off powersave: ", err)
		}
	}

	apOps := []hostapd.Option{
		hostapd.Mode(hostapd.Mode80211nMixed),
		hostapd.Channel(64),
		hostapd.HTCaps(hostapd.HTCapHT20),
	}
	ap, err := tf.ConfigureAP(ctx, apOps, nil)
	if err != nil {
		s.Fatal("Failed to configure AP: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, ap); err != nil {
			s.Error("Failed to deconfig AP: ", err)
		}
	}(ctx)
	s.Log("AP setup done")
	ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap)
	defer cancel()

	// Connect to the initial AP.
	var servicePath string
	if resp, err := tf.ConnectWifiAP(ctx, ap); err != nil {
		s.Fatal("DUT: failed to connect to WiFi: ", err)
	} else {
		servicePath = resp.ServicePath
	}
	defer func(ctx context.Context) {
		if err := tf.DisconnectWifi(ctx); err != nil {
			// Do not fail on this error as we're triggering some
			// disconnection in this test and the service can be
			// inactive at this point.
			s.Log("Failed to disconnect WiFi: ", err)
		}
		req := &network.DeleteEntriesForSSIDRequest{Ssid: []byte(ap.Config().SSID)}
		if _, err := tf.WifiClient().DeleteEntriesForSSID(ctx, req); err != nil {
			s.Errorf("Failed to remove entries for ssid=%s, err: %v", ap.Config().SSID, err)
		}
	}(ctx)
	ctx, cancel = ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	s.Log("Connected")

	// Assert connection.
	if err := tf.PingFromDUT(ctx, ap.ServerIP().String()); err != nil {
		s.Fatal("Failed to ping from DUT: ", err)
	}

	props := []*wificell.ShillProperty{
		&wificell.ShillProperty{
			Property:       shillconst.ServicePropertyState,
			ExpectedValues: []interface{}{shillconst.ServiceStateConfiguration},
			Method:         network.ExpectShillPropertyRequest_ON_CHANGE,
		},
		&wificell.ShillProperty{
			Property:       shillconst.ServicePropertyState,
			ExpectedValues: shillconst.ServiceConnectedStates,
			Method:         network.ExpectShillPropertyRequest_ON_CHANGE,
		},
	}

	waitCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	waitForProps, err := tf.ExpectShillProperty(waitCtx, servicePath, props)
	if err != nil {
		s.Fatal("DUT: failed to create a property watcher, err: ", err)
	}

	const alterChannel = 36
	// Router send CSA.
	if err := tf.Router().SendCSA(ctx, ap, alterChannel); err != nil {
		s.Fatal("Failed to send CSA from AP: ", err)
	}
	s.Log("CSA frame was sent from the AP")

	if err := waitForProps(); err != nil {
		s.Fatal("DUT: failed to wait for the properties, err: ", err)
	}

	// Check frequency.
	service, err := tf.QueryService(ctx)
	if err != nil {
		s.Fatal("Failed to query shill service information: ", err)
	}
	clientFreq := service.Wifi.Frequency
	serverFreq, err := hostapd.ChannelToFrequency(alterChannel)
	if err != nil {
		s.Fatal("Failed to get server frequency")
	}
	if clientFreq != uint32(serverFreq) {
		s.Fatalf("Frequency does not match, got %d want %d: ", clientFreq, serverFreq)
	}
}
