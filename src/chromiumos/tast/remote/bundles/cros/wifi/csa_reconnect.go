// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/network/iw"
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

	const (
		primaryChannel = 64
		alterChannel   = 36
		maxRetry       = 5
	)

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
	ctx, cancel = tf.ReserveForDeconfigAP(ctx, ap)
	defer cancel()

	// Connect to the AP.
	if _, err := tf.ConnectWifiAP(ctx, ap); err != nil {
		s.Fatal("DUT: failed to connect to WiFi: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DisconnectWifi(ctx); err != nil {
			// Do not fail on this error as if the DUT does not
			// support CSA, a disconnection is triggered and the
			// service can be inactive at this point.
			s.Log("Failed to disconnect WiFi: ", err)
		}
		req := &network.DeleteEntriesForSSIDRequest{Ssid: []byte(ap.Config().SSID)}
		if _, err := tf.WifiClient().DeleteEntriesForSSID(ctx, req); err != nil {
			s.Errorf("Failed to remove entries for ssid=%s, err: %v", ap.Config().SSID, err)
		}
	}(ctx)
	s.Log("Connected")
	ctx, cancel = tf.ReserveForDisconnect(ctx)
	defer cancel()

	// Assert connection.
	if err := tf.VerifyConnection(ctx, ap); err != nil {
		s.Fatal("Failed to verify connection: ", err)
	}

	// Start an iw event logger.
	ew, err := iw.NewEventWatcher(ctx, s.DUT())
	if err != nil {
		s.Fatal("Failed to start iw.EventWatcher: ", err)
	}
	defer ew.Stop(ctx)

	// Action frame might be lost, give it some retries.
	for i := 0; i < maxRetry; i++ {
		s.Logf("Try sending channel switch frame %d", i)
		// Router send CSA.
		if err := tf.Router().SendCSA(ctx, ap, alterChannel); err != nil {
			s.Fatal("Failed to send CSA from AP: ", err)
		}
		s.Log("CSA frame was sent from the AP")

		// The frame might need some time to reach DUT, wait for a few seconds.
		wCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		// TODO(b/154879577): Find some way to know if DUT supports
		// channel switch, and only wait for the proper event.
		event, err := ew.WaitByType(wCtx, iw.EventTypeChanSwitch, iw.EventTypeDisconnect)
		if err == context.DeadlineExceeded {
			// Retry if deadline exceeded.
			continue
		}
		if err != nil {
			s.Fatal("Failed to wait for iw event: ", err)
		}
		if event.Type == iw.EventTypeDisconnect {
			s.Fatal("Client disconnection detected")
		}

		s.Log("Channel switch detected")
		break
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Check  if the DUT has moved to the alternate channel.
		chConfig, err := iwr.RadioConfig(ctx, iface)
		if err != nil {
			return errors.Wrap(err, "failed to get the radio configuration")
		}
		if chConfig.Number == alterChannel {
			s.Logf("DUT: Switched  to the alternate channel %d", alterChannel)
			return nil
		}

		return errors.Errorf("DUT: failed to switch to the alternate channel %s", alterChannel)
	}, &testing.PollOptions{
		Timeout:  3 * time.Second,
		Interval: 200 * time.Millisecond,
	}); err != nil {
		s.Fatal("DUT: failed to process the CSA request, err: ", err)
	}

	// Assert connection.
	if err := tf.VerifyConnection(ctx, ap); err != nil {
		s.Fatal("Failed to verify connection: ", err)
	}
}
