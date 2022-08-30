// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"

	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/dutcfg"
	"chromiumos/tast/remote/wificell/tethering"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type sapSimpleConnectTestcase struct {
	tetheringOpts []tethering.Option
	secConfFac    security.ConfigFactory
}

func init() {
	testing.AddTest(&testing.Test{
		Func: SAPSimpleConnect,
		Desc: "Verifies that DUT can start a Soft AP interface and STAs with different WiFi configurations can connect to the DUT",
		Contacts: []string{
			"jintaolin@google.com",            // Test author
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:         []string{"group:wificell_cross_device", "wificell_cross_device_sap", "wificell_cross_device_unstable"},
		ServiceDeps:  []string{wificell.TFServiceName},
		HardwareDeps: hwdep.D(hwdep.WifiSAP()),
		Fixture:      "wificellFixtCompanionDut",
		Params: []testing.Param{
			{
				// Verifies that Soft AP DUT can accept connection from a station with no encryption in low band and high band.
				Name: "open",
				Val: []sapSimpleConnectTestcase{{
					tetheringOpts: []tethering.Option{tethering.Band(tethering.Band2p4g), tethering.NoUplink(true)},
				}, {
					tetheringOpts: []tethering.Option{tethering.Band(tethering.Band5g), tethering.NoUplink(true)},
				}},
			},
			{
				// Verifies that Soft AP DUT can accept connection from a station with WPA2 PSK encryption in low band and high band.
				Name: "wpa2_psk",
				Val: []sapSimpleConnectTestcase{{
					tetheringOpts: []tethering.Option{tethering.Band(tethering.Band2p4g), tethering.NoUplink(true),
						tethering.SecMode(wpa.ModePureWPA2)},
					secConfFac: wpa.NewConfigFactory(
						"chromeos", wpa.Mode(wpa.ModePureWPA2), wpa.Ciphers2(wpa.CipherCCMP),
					),
				}, {
					tetheringOpts: []tethering.Option{tethering.Band(tethering.Band5g), tethering.NoUplink(true),
						tethering.SecMode(wpa.ModePureWPA2)},
					secConfFac: wpa.NewConfigFactory(
						"chromeos", wpa.Mode(wpa.ModePureWPA2), wpa.Ciphers2(wpa.CipherCCMP),
					),
				}},
			},
			{
				// Verifies that Soft AP DUT can accept connection from a station with WPA3 PSK encryption in low band and high band.
				Name:              "wpa3_psk",
				ExtraSoftwareDeps: []string{"wpa3_sae"},
				Val: []sapSimpleConnectTestcase{{
					tetheringOpts: []tethering.Option{tethering.Band(tethering.Band2p4g), tethering.NoUplink(true),
						tethering.SecMode(wpa.ModePureWPA3)},
					secConfFac: wpa.NewConfigFactory(
						"chromeos", wpa.Mode(wpa.ModePureWPA3), wpa.Ciphers2(wpa.CipherCCMP),
					),
				}, {
					tetheringOpts: []tethering.Option{tethering.Band(tethering.Band5g), tethering.NoUplink(true),
						tethering.SecMode(wpa.ModePureWPA3)},
					secConfFac: wpa.NewConfigFactory(
						"chromeos", wpa.Mode(wpa.ModePureWPA3), wpa.Ciphers2(wpa.CipherCCMP),
					),
				}},
			},
			{
				// Verifies that Soft AP DUT can accept connection from a station with WPA3 transitional PSK encryption in low band and high band.
				Name:              "wpa3mixed_psk",
				ExtraSoftwareDeps: []string{"wpa3_sae"},
				Val: []sapSimpleConnectTestcase{{
					tetheringOpts: []tethering.Option{tethering.Band(tethering.Band2p4g), tethering.NoUplink(true),
						tethering.SecMode(wpa.ModeMixedWPA3)},
					secConfFac: wpa.NewConfigFactory(
						"chromeos", wpa.Mode(wpa.ModeMixedWPA3), wpa.Ciphers2(wpa.CipherCCMP),
					),
				}, {
					tetheringOpts: []tethering.Option{tethering.Band(tethering.Band5g), tethering.NoUplink(true),
						tethering.SecMode(wpa.ModeMixedWPA3)},
					secConfFac: wpa.NewConfigFactory(
						"chromeos", wpa.Mode(wpa.ModeMixedWPA3), wpa.Ciphers2(wpa.CipherCCMP),
					),
				}},
			},
		},
	})
}

func SAPSimpleConnect(ctx context.Context, s *testing.State) {
	/*
		This test checks the soft AP connection of the chromebook by using
		the following steps:
		1- Configures the main DUT as a soft AP.
		2- Configures the Companion DUT as a STA.
		3- Connects the the STA to the soft AP.
		4- Verify the connection by running ping from the STA.
		5- Deconfigure the STA.
		6- Deconfigure the soft AP.
	*/
	tf := s.FixtValue().(*wificell.TestFixture)
	if tf.NumberOfDUTs() < 2 {
		s.Fatal("Test requires at least 2 DUTs to be declared. Only have ", tf.NumberOfDUTs())
	}

	testOnce := func(ctx context.Context, s *testing.State, options []tethering.Option, fac security.ConfigFactory) {
		tetheringConf, _, err := tf.StartTethering(ctx, wificell.DefaultDUT, options, fac)
		if err != nil {
			s.Fatal("Failed to start tethering session on DUT, err: ", err)
		}

		defer func(ctx context.Context) {
			if err := tf.StopTethering(ctx, wificell.DefaultDUT); err != nil {
				s.Error("Failed to stop tethering session on DUT, err: ", err)
			}
		}(ctx)
		ctx, cancel := tf.ReserveForStopTethering(ctx)
		defer cancel()
		s.Log("Tethering session started")

		cdIdx := wificell.DutIdx(1)
		_, err = tf.ConnectWifiFromDUT(ctx, cdIdx, tetheringConf.SSID, dutcfg.ConnSecurity(tetheringConf.SecConf))
		if err != nil {
			s.Fatal("Failed to connect to Soft AP, err: ", err)
		}
		defer func(ctx context.Context) {
			if err := tf.DisconnectDUTFromWifi(ctx, cdIdx); err != nil {
				s.Error("Failed to disconnect from Soft AP, err: ", err)
			}
		}(ctx)
		ctx, cancel = tf.ReserveForDisconnect(ctx)
		defer cancel()
		s.Log("Connected")

		addrs, err := tf.DUTIPv4Addrs(ctx, wificell.DefaultDUT)
		if err != nil || len(addrs) == 0 {
			s.Fatal("Failed to get the Soft AP's IP address: ", err)
		}
		if _, err := tf.PingFromSpecificDUT(ctx, cdIdx, addrs[0].String()); err != nil {
			s.Fatal("Failed to ping from Companion DUT to DUT: ", err)
		}
	}

	testcases := s.Param().([]sapSimpleConnectTestcase)
	for i, tc := range testcases {
		subtest := func(ctx context.Context, s *testing.State) {
			testOnce(ctx, s, tc.tetheringOpts, tc.secConfFac)
		}
		s.Run(ctx, fmt.Sprintf("Testcase #%d", i), subtest)
	}
	s.Log("Tearing down")
}
