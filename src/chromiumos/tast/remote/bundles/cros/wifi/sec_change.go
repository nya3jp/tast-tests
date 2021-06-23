// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/hostapd"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: SecChange,
		Desc: "Verifies that the DUT can connect to a BSS despite security changes",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_cq"},
		ServiceDeps: []string{wificell.TFServiceName},
		Fixture:     "wificellFixtWithCapture",
	})
}

func SecChange(ctx context.Context, s *testing.State) {
	tf := s.FixtValue().(*wificell.TestFixture)

	// setUpAndConnect sets up the WiFi AP and verifies the DUT can connect to it.
	setUpAndConnect := func(ctx context.Context, options []hostapd.Option, fac security.ConfigFactory) (retErr error) {
		ctx, st := timing.Start(ctx, "setUpAndConnect")
		defer st.End()

		// collectErr logs the given err and returns it if the returning
		// error is is unspecified. Used in deferred functions.
		collectErr := func(err error) {
			s.Log("Error in setUpAndConnect: ", err)
			if retErr == nil {
				retErr = err
			}
		}
		ap, err := tf.ConfigureAP(ctx, options, fac)
		if err != nil {
			return errors.Wrap(err, "failed to configure ap")
		}
		defer func(ctx context.Context) {
			s.Log("Deconfiguring AP")
			if err := tf.DeconfigAP(ctx, ap); err != nil {
				collectErr(errors.Wrap(err, "failed to deconfig ap"))
			}
		}(ctx)
		ctx, cancel := tf.ReserveForDeconfigAP(ctx, ap)
		defer cancel()
		s.Log("AP setup done")

		if _, err := tf.ConnectWifiAP(ctx, ap); err != nil {
			return errors.Wrap(err, "failed to connect to WiFi")
		}
		defer func(ctx context.Context) {
			s.Log("Disconnecting")
			if err := tf.DisconnectWifi(ctx); err != nil {
				collectErr(errors.Wrap(err, "failed to disconnect"))
			}
			// Leave the profile entry as is, as we're going to verify
			// the behavior with it in next call.
		}(ctx)
		ctx, cancel = tf.ReserveForDisconnect(ctx)
		defer cancel()
		s.Log("Connected")

		if err := tf.PingFromDUT(ctx, ap.ServerIP().String()); err != nil {
			return errors.Wrap(err, "failed to ping server from DUT")
		}
		return nil
	}

	apOps := []hostapd.Option{
		hostapd.SSID("TAST_TEST_SecChange"),
		hostapd.Mode(hostapd.Mode80211nMixed),
		hostapd.Channel(48),
		hostapd.HTCaps(hostapd.HTCapHT40),
	}
	wpaOps := []wpa.Option{
		wpa.Mode(wpa.ModeMixed),
		wpa.Ciphers(wpa.CipherTKIP, wpa.CipherCCMP),
		wpa.Ciphers2(wpa.CipherCCMP),
	}
	wpaFac := wpa.NewConfigFactory("chromeos", wpaOps...)
	// Try connecting to a protected network (WPA).
	if err := setUpAndConnect(ctx, apOps, wpaFac); err != nil {
		s.Fatal("Failed to connect to a protected network (WPA): ", err)
	}
	// Assert that we can still connect to the open network with the same SSID.
	if err := setUpAndConnect(ctx, apOps, nil); err != nil {
		s.Fatal("Failed to connect to the open network: ", err)
	}
}
