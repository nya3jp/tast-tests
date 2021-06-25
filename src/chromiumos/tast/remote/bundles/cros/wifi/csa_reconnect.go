// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CSAReconnect,
		Desc: "Verifies that DUT will switch to the new channel after the AP starts a CSA",
		Contacts: []string{
			"arowa@google.com",                // Test author
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixt",
	})
}

func CSAReconnect(ctx context.Context, s *testing.State) {
	const (
		primaryChannel = 64
		alterChannel   = 36
	)

	tf := s.FixtValue().(*wificell.TestFixture)

	apOps := []hostapd.Option{hostapd.Mode(hostapd.Mode80211nMixed), hostapd.Channel(primaryChannel), hostapd.HTCaps(hostapd.HTCapHT20)}
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
	ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap)
	defer cancel()

	// Connect to the AP.
	var servicePath string
	ctxForDisconnect := ctx
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()
	if resp, err := tf.ConnectWifiAP(ctx, ap); err != nil {
		s.Fatal("DUT: failed to connect to WiFi: ", err)
	} else {
		servicePath = resp.ServicePath
	}

	defer func(ctx context.Context) {
		if err := tf.CleanDisconnectWifi(ctx); err != nil {
			s.Error("DUT: failed to disconnect WiFi: ", err)
		}
	}(ctxForDisconnect)
	s.Log("Connected")

	// Assert connection.
	if err := tf.VerifyConnection(ctx, ap); err != nil {
		s.Fatal("Failed to verify connection: ", err)
	}

	alterFreq, err := hostapd.ChannelToFrequency(alterChannel)
	if err != nil {
		s.Fatal("Failed to get server frequency: ", err)
	}
	props := []*wificell.ShillProperty{
		&wificell.ShillProperty{
			Property:       shillconst.ServicePropertyWiFiFrequency,
			ExpectedValues: []interface{}{uint32(alterFreq)},
			Method:         wifi.ExpectShillPropertyRequest_CHECK_WAIT,
		},
	}

	waitCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	monitorProps := []string{shillconst.ServicePropertyIsConnected}
	waitForProps, err := tf.ExpectShillProperty(waitCtx, servicePath, props, monitorProps)
	if err != nil {
		s.Fatal("DUT: failed to create a property watcher, err: ", err)
	}

	// Router starts ChannelSwitching.
	csaStart := time.Now()
	if err := ap.StartChannelSwitch(ctx, 8, alterChannel, hostapd.CSAMode("ht")); err != nil {
		s.Fatal("Failed to send CSA from AP: ", err)
	}
	s.Log("CSA frame was sent from the AP")

	monitorResult, err := waitForProps()
	if err != nil {
		s.Fatal("DUT: failed to wait for the properties, err: ", err)
	}
	s.Log("DUT: switched channel")

	// Assert there was no disconnection during channel switching.
	for _, ph := range monitorResult {
		if ph.Name == shillconst.ServicePropertyIsConnected {
			if !ph.Value.(bool) {
				// TODO(b/181365077): Gale AP takes a long time to send beacons on the new channel and that
				// causes the DUT to abort the CSA process. After this issue is fixed, the test should fail
				// if a disconnection is detected.
				s.Log("DUT: failed to stay connected during the channel switching process")
			}
		}
	}

	// Assert connection.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		return tf.VerifyConnection(ctx, ap)
	}, &testing.PollOptions{
		Timeout:  20 * time.Second,
		Interval: time.Second,
	}); err != nil {
		s.Fatal("Failed to verify connection: ", err)
	}

	s.Log("Connection verified after CSA; elapsed time:", time.Since(csaStart))
}
