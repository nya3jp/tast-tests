// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LinkMonitorFailure,
		Desc: "Verifies how fast the DUT detects the link failure and reconnects to the AP when an AP changes its DHCP configuration",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_func"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixt",
	})
}

func LinkMonitorFailure(ctx context.Context, s *testing.State) {
	const (
		// Passive link monitor takes up to 25 seconds to fail; active link monitor takes up to 50 seconds to fail.
		linkFailureDetectedTimeout = 80 * time.Second
		reassociateTimeout         = 10 * time.Second
	)

	tf := s.FixtValue().(*wificell.TestFixture)

	ap, err := tf.DefaultOpenNetworkAP(ctx)
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
	s.Log("Test fixture setup done; connecting the DUT to the AP")

	if _, err := tf.ConnectWifiAP(ctx, ap); err != nil {
		s.Fatal("Failed to connect to WiFi: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DisconnectWifi(ctx); err != nil {
			s.Error("Failed to disconnect WiFi: ", err)
		}
		req := &wifi.DeleteEntriesForSSIDRequest{Ssid: []byte(ap.Config().SSID)}
		if _, err := tf.WifiClient().DeleteEntriesForSSID(ctx, req); err != nil {
			s.Errorf("Failed to remove entries for ssid=%s: %v", ap.Config().SSID, err)
		}
	}(ctx)
	ctx, cancel = ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()
	s.Log("WiFi connected; starting the test")

	if err := tf.PingFromDUT(ctx, ap.ServerIP().String()); err != nil {
		s.Fatal("Failed to ping from the DUT: ", err)
	}

	ew, err := iw.NewEventWatcher(ctx, s.DUT())
	if err != nil {
		s.Fatal("Failed to create iw event watcher: ", err)
	}
	defer ew.Stop(ctx)

	// Start to change the DHCP config.

	// Obtain current time from the DUT because we use the "disconnect" event timestamp as
	// the end time of the link failure detection duration, which is from the DUT's clock.
	linkFailureTime, err := tf.CurrentClientTime(ctx)
	if err != nil {
		s.Fatal("Failed to get the current DUT time: ", err)
	}
	if err := ap.ChangeSubnetIdx(ctx); err != nil {
		s.Fatal("Failed to change the subnet index of the AP: ", err)
	}

	s.Log("Waiting for link failure detected event")
	wCtx, cancel := context.WithTimeout(ctx, linkFailureDetectedTimeout)
	defer cancel()
	linkFailureDetectedEv, err := ew.WaitByType(wCtx, iw.EventTypeDisconnect)
	if err != nil {
		s.Fatal("Failed to wait for link failure detected event: ", err)
	}

	// Calculate duration for sensing the link failure.
	linkFailureDetectedDuration := linkFailureDetectedEv.Timestamp.Sub(linkFailureTime)
	if linkFailureDetectedDuration > linkFailureDetectedTimeout {
		s.Error("Failed to detect link failure within given timeout")
	}
	s.Logf("Link failure detection time: %.2f seconds", linkFailureDetectedDuration.Seconds())

	s.Log("Waiting for reassociation to complete")
	wCtx, cancel = context.WithTimeout(ctx, reassociateTimeout)
	defer cancel()
	connectedEv, err := ew.WaitByType(wCtx, iw.EventTypeConnected)
	if err != nil {
		s.Error("Failed to wait for reassociation to complete: ", err)
	}

	// Get the reassociation time.
	reassociateDuration := connectedEv.Timestamp.Sub(linkFailureDetectedEv.Timestamp)
	if reassociateDuration < 0 {
		s.Errorf("Unexpected reassociate duration: %d is negative", reassociateDuration)
	}
	if reassociateDuration > reassociateTimeout {
		s.Error("Failed to reassociate within given timeout")
	}
	s.Logf("Reassociate time: %.2f seconds", reassociateDuration.Seconds())
}
