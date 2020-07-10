// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/network/iw"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/services/cros/network"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:        LinkMonitorFailure,
		Desc:        "Verifies that how fast the DUT detects the link failure and reconnects to the AP when an AP changes its DHCP configuration",
		Contacts:    []string{"chharry@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_unstable", "wificell_func"},
		ServiceDeps: []string{"tast.cros.network.WifiService"},
		Vars:        []string{"router"},
	})
}

func LinkMonitorFailure(ctx context.Context, s *testing.State) {
	const (
		// Passive link monitor takes up to 25 seconds to fail; active link monitor takes up to 50 seconds to fail.
		linkFailureTimeout = 80 * time.Second
		reassociateTimeout = 10 * time.Second
	)

	router, _ := s.Var("router")
	tf, err := wificell.NewTestFixture(ctx, ctx, s.DUT(), s.RPCHint(), wificell.TFRouter(router))
	if err != nil {
		s.Fatal("Failed to set up test fixture: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.Close(ctx); err != nil {
			s.Log("Failed to tear down test fixture: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForClose(ctx)
	defer cancel()

	ap, err := tf.DefaultOpenNetworkAP(ctx)
	if err != nil {
		s.Fatal("Failed to configure the AP: ", err)
	}
	defer func(ctx context.Context) {
		if err := tf.DeconfigAP(ctx, ap); err != nil {
			s.Error("Failed to deconfig the AP: ", err)
		}
	}(ctx)
	ctx, _ = tf.ReserveForDeconfigAP(ctx, ap)
	s.Log("AP setup done")

	if _, err := tf.ConnectWifiAP(ctx, ap); err != nil {
		s.Fatal("Failed to connect to WiFi: ", err)
	}
	defer func() {
		if err := tf.DisconnectWifi(ctx); err != nil {
			s.Error("Failed to disconnect WiFi: ", err)
		}
		req := &network.DeleteEntriesForSSIDRequest{Ssid: []byte(ap.Config().SSID)}
		if _, err := tf.WifiClient().DeleteEntriesForSSID(ctx, req); err != nil {
			s.Errorf("Failed to remove entries for ssid=%s: %v", ap.Config().SSID, err)
		}
	}()
	s.Log("Connected")

	if err := tf.PingFromDUT(ctx, ap.ServerIP().String()); err != nil {
		s.Fatal("Failed to ping from the DUT: ", err)
	}

	el, err := iw.NewEventLogger(ctx, s.DUT())
	if err != nil {
		s.Fatal("Failed to create iw event logger")
	}
	defer el.Stop(ctx)

	// Start to change the DHCP config.
	startTime := time.Now()
	if err := tf.Router().ChangeAPIfaceSubnetIdx(ctx, ap); err != nil {
		s.Fatal("Failed to change the subnet index of AP: ", err)
	}

	s.Log("Waiting for link failure and reassociation to complete")
	if err := testing.Sleep(ctx, linkFailureTimeout+reassociateTimeout); err != nil {
		s.Fatal("Failed to wait for link failure and reassociation to complete: ", err)
	}

	// Get the link failure detection time.
	if disconnectEvs := el.EventsByType(iw.EventTypeDisconnect); len(disconnectEvs) == 0 {
		// Some drivers perform a true reassociation without disconnect. See also https://crbug.com/990012.
		s.Log("Failed to disconnect within timeout; this is expected for some drivers")
	} else {
		linkFailureTime := disconnectEvs[0].Timestamp.Sub(startTime)
		if linkFailureTime > linkFailureTimeout {
			s.Error("Failed to detect link failure within given timeout")
		}
		s.Logf("Link failure detection time: %.2f seconds", linkFailureTime.Seconds())
	}

	// Get the reassociation time.
	rt, err := reassociateTime(el)
	if err != nil {
		s.Fatal("Failed to get reassociate time: ", err)
	}
	if rt > reassociateTimeout {
		s.Error("Failed to reassociate within given timeout")
	}
	s.Logf("Reassociate time: %.2f seconds", rt.Seconds())
}

func reassociateTime(el *iw.EventLogger) (time.Duration, error) {
	var reassociateStart time.Time
	if startScanEvs := el.EventsByType(iw.EventTypeScanStart); len(startScanEvs) != 0 {
		reassociateStart = startScanEvs[0].Timestamp
	}
	// Newer wpa_supplicant would attempt to disconnect then reconnect without scanning.
	// So if no scan event is detected before the disconnect attempt,
	// we'll assume the disconnect attempt is the beginning of the reassociate attempt.
	disconnectEvs := el.EventsByType(iw.EventTypeDisconnect)
	if len(disconnectEvs) != 0 {
		if reassociateStart.IsZero() || reassociateStart.After(disconnectEvs[0].Timestamp) {
			reassociateStart = disconnectEvs[0].Timestamp
		}
	}
	if reassociateStart.IsZero() {
		return 0, errors.New("failed to get reassociate start time")
	}

	connectedEvs := el.EventsByType(iw.EventTypeConnected)
	if len(connectedEvs) == 0 {
		return 0, errors.New("failed to get reassociate end time")
	}
	return connectedEvs[0].Timestamp.Sub(reassociateStart), nil
}
