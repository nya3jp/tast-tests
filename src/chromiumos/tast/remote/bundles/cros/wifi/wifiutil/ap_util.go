// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifiutil

import (
	"context"

	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/testing"
)

// ConfigureAP is a helper wrapper on TestFixture.ConfigureAP that additionally
// handles error, returns deconfig function and provide channel frequency the AP is configured for.
// Requires s.FixtValue() to return *wificell.TextFixture.
// Calls s.Fatal in case of any error during setup.
func ConfigureAP(ctx context.Context, s *testing.State, apParams []hostapd.Option, routerIdx int,
	secConfFac security.ConfigFactory) (ap *wificell.APIface, freq int, deconfig func(context.Context, *wificell.APIface) error) {

	tf := s.FixtValue().(*wificell.TestFixture)

	ap, err := tf.ConfigureAPOnRouterID(ctx, routerIdx, apParams, secConfFac)
	if err != nil {
		s.Fatalf("Failed to configure AP%d, err: %s", routerIdx, err)
	}
	s.Logf("AP%d setup done", routerIdx)

	deconfig = func(ctx context.Context, ap *wificell.APIface) error {
		if err := tf.DeconfigAP(ctx, ap); err != nil {
			s.Errorf("Failed to deconfig AP, err: %s", err)
			return err
		}
		s.Logf("AP%d teardown done", routerIdx)
		return nil
	}

	freq, err = hostapd.ChannelToFrequency(ap.Config().Channel)
	if err != nil {
		deconfig(ctx, ap)
		s.Fatalf("Failed to get frequency for channel %d: %v", ap.Config().Channel, err)
	}

	return ap, freq, deconfig
}

// ConnectAP is a helper wrapper on TestFixture.ConnectWifiAP that additionally
// handles errors and returns disconnect function.
// Requires s.FixtValue() to return *wificell.TextFixture.
// Calls s.Fatal in case of any error during connect.
func ConnectAP(ctx context.Context, s *testing.State, ap *wificell.APIface, apIdx int) (disconnect func(context.Context)) {
	tf := s.FixtValue().(*wificell.TestFixture)

	if _, err := tf.ConnectWifiAP(ctx, ap); err != nil {
		s.Fatal("Failed to connect to WiFi, err: ", err)
	}

	s.Log("Connected to AP", apIdx)

	return func(ctx context.Context) {
		if err := tf.CleanDisconnectWifi(ctx); err != nil {
			s.Error("Failed to disconnect WiFi, err: ", err)
		}
	}
}
