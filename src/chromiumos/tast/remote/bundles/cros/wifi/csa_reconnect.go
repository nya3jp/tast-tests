// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/network/iw"
	remoteiw "chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        CSAReconnect,
		Desc:        "Verifies that DUT will switch to the new channel after the AP starts a CSA",
		Contacts:    []string{"arowa@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{wificell.TFServiceName},
		Pre:         wificell.TestFixturePre(),
		Vars:        []string{"router", "pcap"},
		// Run the test only on boards that support CSA.
		HardwareDeps: hwdep.D(hwdep.Platform("kukui", "jacuzzi", "grunt", "zork", "dedede", "speedy")),
	})
}

func CSAReconnect(ctx context.Context, s *testing.State) {
	tf := s.PreValue().(*wificell.TestFixture)
	defer func(ctx context.Context) {
		if err := tf.CollectLogs(ctx); err != nil {
			s.Log("Error collecting logs, err: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForCollectLogs(ctx)
	defer cancel()

	const (
		primaryChannel = 64
		alterChannel   = 36
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
		if err := tf.CleanDisconnectWifi(ctx); err != nil {
			s.Error("DUT: failed to disconnect WiFi: ", err)
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

	// Router send CSA.
	if err := ap.ChannelSwitch(ctx, alterChannel, 8, "ht"); err != nil {
		s.Fatal("Failed to send CSA from AP: ", err)
	}
	s.Log("CSA frame was sent from the AP")

	// The frame might need some time to reach DUT, wait for a few seconds.
	wCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	event, err := ew.WaitByType(wCtx, iw.EventTypeChanSwitch, iw.EventTypeDisconnect)
	if err != nil {
		s.Fatal("Failed to wait for iw event: ", err)
	}
	if event.Type == iw.EventTypeDisconnect {
		s.Fatal("Client disconnection detected")
	}

	iwr := remoteiw.NewRemoteRunner(s.DUT().Conn())
	iface, err := tf.ClientInterface(ctx)
	if err != nil {
		s.Fatal("Failed to get the client interface: ", err)
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		// Check if the DUT has moved to the alternate channel.
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
}
