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
		Func:        SecChange,
		Desc:        "Verifies that the DUT can connect to a BSS despite security changes",
		Contacts:    []string{"yenlinlai@google.com", "chromeos-platform-connectivity@google.com"},
		Attr:        []string{"group:wificell", "wificell_func", "wificell_unstable"},
		ServiceDeps: []string{"tast.cros.network.WifiService"},
		Vars:        []string{"router", "pcap"},
	})
}

func SecChange(fullCtx context.Context, s *testing.State) {
	var tfOps []wificell.TFOption
	if router, _ := s.Var("router"); router != "" {
		tfOps = append(tfOps, wificell.TFRouter(router))
	}
	if pcap, _ := s.Var("pcap"); pcap != "" {
		tfOps = append(tfOps, wificell.TFPcap(pcap))
	}

	tf, err := wificell.NewTestFixture(fullCtx, fullCtx, s.DUT(), s.RPCHint(), tfOps...)
	if err != nil {
		s.Fatal("Failed to set up test fixture: ", err)
	}
	defer func() {
		if err := tf.Close(fullCtx); err != nil {
			s.Log("Failed to tear down test fixture, err: ", err)
		}
	}()

	ctx, cancel := tf.ReserveForClose(fullCtx)
	defer cancel()

	// setUpAndConnect sets up the WiFi AP and verifies the DUT can connect to it.
	setUpAndConnect := func(fullCtx context.Context, options []hostapd.Option, fac security.ConfigFactory) (retErr error) {
		fullCtx, st := timing.Start(fullCtx, "setUpAndConnect")
		defer st.End()

		// collectErr logs the given err and returns it if the returning
		// error is is unspecified. Used in deferred functions.
		collectErr := func(err error) {
			s.Log("Error in setUpAndConnect: ", err)
			if retErr == nil {
				retErr = err
			}
		}
		ap, err := tf.ConfigureAP(fullCtx, options, fac)
		if err != nil {
			return errors.Wrap(err, "failed to configure ap")
		}
		defer func() {
			s.Log("Deconfiguring AP")
			if err := tf.DeconfigAP(fullCtx, ap); err != nil {
				collectErr(errors.Wrap(err, "failed to deconfig ap"))
			}
		}()
		ctx, cancel := tf.ReserveForDeconfigAP(fullCtx, ap)
		defer cancel()
		s.Log("AP setup done")

		if _, err := tf.ConnectWifi(ctx, ap); err != nil {
			return errors.Wrap(err, "failed to connect to WiFi")
		}
		defer func() {
			s.Log("Disconnecting")
			if err := tf.DisconnectWifi(fullCtx); err != nil {
				collectErr(errors.Wrap(err, "failed to disconnect"))
			}
			// Leave the profile entry as is, as we're going to verify
			// the behavior with it in next call.
		}()
		s.Log("Connected")

		if err := tf.PingFromDUT(ctx); err != nil {
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
