// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/dutcfg"
	"chromiumos/tast/remote/wificell/router"
	"chromiumos/tast/remote/wificell/security"
	"chromiumos/tast/remote/wificell/security/axbase"
	"chromiumos/tast/remote/wificell/security/axwpa"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AxSimpleConnect,
		Desc: "Verifies that DUT can connect to the host via AP in different WiFi configuration",
		Contacts: []string{
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{"group:wificell", "wificell_func"},
		ServiceDeps: []string{wificell.TFServiceName},
		Vars:        []string{"router", "pcap"},
		Params: []testing.Param{
			{
				// Verifies that DUT can connect to an open 802.11ax on channels 48, 52
				Name: "80211axopen",
				Val: []axSimpleConnectTestcase{{
					band:       router.Wl1,
					apOpts:     []router.Option{router.Mode(router.Mode80211ax), router.Channel(48), router.Ssid("googleTest0"), router.Hidden(false), router.ChanBandwidth(router.Bw80)},
					secConfFac: axbase.NewConfigFactory(router.Wl1),
				}, {
					band:       router.Wl1,
					apOpts:     []router.Option{router.Mode(router.Mode80211ax), router.Channel(52), router.Ssid("googleTest1"), router.Hidden(false), router.ChanBandwidth(router.Bw80)},
					secConfFac: axbase.NewConfigFactory(router.Wl1),
				}},
			},
			{
				// Verifies that DUT can connect to a hidden 802.11ax network on channel 48, 52
				Name: "80211axopenhidden",
				Val: []axSimpleConnectTestcase{{
					band:       router.Wl1,
					apOpts:     []router.Option{router.Mode(router.Mode80211ax), router.Channel(48), router.Ssid("googleTest0"), router.Hidden(true), router.ChanBandwidth(router.Bw80)},
					secConfFac: axbase.NewConfigFactory(router.Wl1),
				}, {
					band:       router.Wl1,
					apOpts:     []router.Option{router.Mode(router.Mode80211ax), router.Channel(52), router.Ssid("googleTest1"), router.Hidden(true), router.ChanBandwidth(router.Bw80)},
					secConfFac: axbase.NewConfigFactory(router.Wl1),
				}},
			},
			{
				// Verifies that DUT can connect to an open wpa (AES) 802.11ax network on channel 48, 52
				Name: "80211axwpaaes",
				Val: []axSimpleConnectTestcase{{
					band:       router.Wl1,
					apOpts:     []router.Option{router.Mode(router.Mode80211ax), router.Channel(48), router.Ssid("googleTest0"), router.Hidden(false), router.ChanBandwidth(router.Bw80)},
					secConfFac: axwpa.NewConfigFactory(router.Wl1, "helloworld", router.AES, wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherCCMP)),
				}, {
					band:       router.Wl1,
					apOpts:     []router.Option{router.Mode(router.Mode80211ax), router.Channel(52), router.Ssid("googleTest1"), router.Hidden(false), router.ChanBandwidth(router.Bw80)},
					secConfFac: axwpa.NewConfigFactory(router.Wl1, "helloworld", router.AES, wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherCCMP)),
				}},
			},
			{
				// Verifies that DUT can connect to an open wpa (AES+TKIP) 802.11ax network on channel 48, 52
				Name: "80211axwpamixed",
				Val: []axSimpleConnectTestcase{{
					band:       router.Wl1,
					apOpts:     []router.Option{router.Mode(router.Mode80211ax), router.Channel(48), router.Ssid("googleTest0"), router.Hidden(true), router.ChanBandwidth(router.Bw80)},
					secConfFac: axwpa.NewConfigFactory(router.Wl1, "helloworld", router.TKIPAES, wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherTKIP, wpa.CipherCCMP)),
				}, {
					band:       router.Wl1,
					apOpts:     []router.Option{router.Mode(router.Mode80211ax), router.Channel(52), router.Ssid("googleTest1"), router.Hidden(true), router.ChanBandwidth(router.Bw80)},
					secConfFac: axwpa.NewConfigFactory(router.Wl1, "helloworld", router.TKIPAES, wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherTKIP, wpa.CipherCCMP)),
				}},
			},
			{
				// Verifies that DUT can connect to a hidden (AES) 802.11ax network on channel 48, 52
				Name: "80211axwpahiddenaes",
				Val: []axSimpleConnectTestcase{{
					band:       router.Wl1,
					apOpts:     []router.Option{router.Mode(router.Mode80211ax), router.Channel(48), router.Ssid("googleTest0"), router.Hidden(true), router.ChanBandwidth(router.Bw80)},
					secConfFac: axwpa.NewConfigFactory(router.Wl1, "helloworld", router.AES, wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherCCMP)),
				}, {
					band:       router.Wl1,
					apOpts:     []router.Option{router.Mode(router.Mode80211ax), router.Channel(52), router.Ssid("googleTest1"), router.Hidden(true), router.ChanBandwidth(router.Bw80)},
					secConfFac: axwpa.NewConfigFactory(router.Wl1, "helloworld", router.AES, wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherCCMP)),
				}},
			},
			{
				// Verifies that DUT can connect to a hidden (AES+TKIP) 802.11ax network on channel 48, 52
				Name: "80211axwpahiddenmixed",
				Val: []axSimpleConnectTestcase{{
					band:       router.Wl1,
					apOpts:     []router.Option{router.Mode(router.Mode80211ax), router.Channel(48), router.Ssid("googleTest0"), router.Hidden(true), router.ChanBandwidth(router.Bw80)},
					secConfFac: axwpa.NewConfigFactory(router.Wl1, "helloworld", router.TKIPAES, wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherTKIP, wpa.CipherCCMP)),
				}, {
					band:       router.Wl1,
					apOpts:     []router.Option{router.Mode(router.Mode80211ax), router.Channel(52), router.Ssid("googleTest1"), router.Hidden(true), router.ChanBandwidth(router.Bw80)},
					secConfFac: axwpa.NewConfigFactory(router.Wl1, "helloworld", router.TKIPAES, wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherTKIP, wpa.CipherCCMP)),
				}},
			},
			{
				// Verifies that DUT can connect to an open 802.11ax on channels 1,6,11 with 20Mhz channel width on the 2ghz band
				Name: "80211axopen20mhz2ghz",
				Val: []axSimpleConnectTestcase{{
					band:       router.Wl0,
					apOpts:     []router.Option{router.Mode(router.Mode80211ax), router.Channel(1), router.Ssid("googleTest0"), router.Hidden(false), router.ChanBandwidth(router.Bw20)},
					secConfFac: axbase.NewConfigFactory(router.Wl0),
				}, {
					band:       router.Wl0,
					apOpts:     []router.Option{router.Mode(router.Mode80211ax), router.Channel(6), router.Ssid("googleTest1"), router.Hidden(false), router.ChanBandwidth(router.Bw20)},
					secConfFac: axbase.NewConfigFactory(router.Wl0),
				}, {
					band:       router.Wl0,
					apOpts:     []router.Option{router.Mode(router.Mode80211ax), router.Channel(11), router.Ssid("googleTest2"), router.Hidden(false), router.ChanBandwidth(router.Bw20)},
					secConfFac: axbase.NewConfigFactory(router.Wl0),
				}},
			},
			{
				// Verifies that DUT can connect to an open 802.11ax on channels 1, 6, 11 with 40Mhz channel width on the 2ghz band
				Name: "80211axopen40mhz2ghz",
				Val: []axSimpleConnectTestcase{{
					band:       router.Wl0,
					apOpts:     []router.Option{router.Mode(router.Mode80211ax), router.Channel(1), router.Ssid("googleTest0"), router.Hidden(false), router.ChanBandwidth(router.Bw40)},
					secConfFac: axbase.NewConfigFactory(router.Wl0),
				}, {
					band:       router.Wl0,
					apOpts:     []router.Option{router.Mode(router.Mode80211ax), router.Channel(6), router.Ssid("googleTest1"), router.Hidden(false), router.ChanBandwidth(router.Bw40)},
					secConfFac: axbase.NewConfigFactory(router.Wl0),
				}, {
					band:       router.Wl0,
					apOpts:     []router.Option{router.Mode(router.Mode80211ax), router.Channel(11), router.Ssid("googleTest2"), router.Hidden(false), router.ChanBandwidth(router.Bw40)},
					secConfFac: axbase.NewConfigFactory(router.Wl0),
				}},
			},
			{
				// Verifies that DUT can connect to an open 802.11ax on channels 48 with 40Mhz channel width on the 5ghz band
				Name: "80211axopen40mhz5ghz",
				Val: []axSimpleConnectTestcase{{
					band:       router.Wl1,
					apOpts:     []router.Option{router.Mode(router.Mode80211ax), router.Channel(48), router.Ssid("googleTest0"), router.Hidden(false), router.ChanBandwidth(router.Bw40)},
					secConfFac: axbase.NewConfigFactory(router.Wl1),
				}},
			},
			{
				// Verifies that DUT can connect to an open 802.11ax on channels 48 with 80Mhz channel width on the 5ghz band
				Name: "80211axopen80mhz5ghz",
				Val: []axSimpleConnectTestcase{{
					band:       router.Wl1,
					apOpts:     []router.Option{router.Mode(router.Mode80211ax), router.Channel(48), router.Ssid("googleTest0"), router.Hidden(false), router.ChanBandwidth(router.Bw80)},
					secConfFac: axbase.NewConfigFactory(router.Wl1),
				}},
			},
			{
				// Verifies that DUT can connect to an open 802.11ax on channels 48 with 160Mhz channel width on the 5ghz band
				Name: "80211axopen160mhz5ghz",
				Val: []axSimpleConnectTestcase{{
					band:       router.Wl1,
					apOpts:     []router.Option{router.Mode(router.Mode80211ax), router.Channel(48), router.Ssid("googleTest0"), router.Hidden(false), router.ChanBandwidth(router.Bw160)},
					secConfFac: axbase.NewConfigFactory(router.Wl1),
				}},
			},
		},
	})
}

type axSimpleConnectTestcase struct {
	apOpts []router.Option
	band   router.BandEnum
	// If unassigned, use default security config: open network.
	secConfFac      security.AxConfigFactory
	pingOps         []ping.Option
	expectedFailure bool
}

func AxSimpleConnect(ctx context.Context, s *testing.State) {
	var tfOps []wificell.TFOption
	if router, ok := s.Var("router"); ok && router != "" {
		tfOps = append(tfOps, wificell.TFRouter(router))
	}
	tfOps = append(tfOps, wificell.TFRouterType(router.AxT))
	// Assert WiFi is up.
	tf, err := wificell.NewTestFixture(ctx, ctx, s.DUT(), s.RPCHint(), tfOps...)
	if err != nil {
		s.Fatal("Failed to set up test fixture: ", err)
	}

	defer func(ctx context.Context) {
		if err := tf.Close(ctx); err != nil {
			s.Error("Failed to properly take down test fixture: ", err)
		}
	}(ctx)
	ctx, cancel := tf.ReserveForClose(ctx)
	defer cancel()

	rt, err := tf.AxRouter()
	if err != nil {
		s.Fatal("Failed to get ax router: ", err)

	}
	if err := rt.SaveConfiguration(ctx); err != nil {
		s.Fatalf("Could not save current router configuration: ", err)
	}
	defer rt.RestoreConfiguration(ctx)

	testOnce := func(ctx context.Context, s *testing.State, band router.BandEnum, options []router.Option, fac security.AxConfigFactory, pingOps []ping.Option, expectedFailure bool) {
		var cfg router.Config
		cfg.Band = band
		for _, opt := range options {
			opt(&cfg)
		}
		var secCfg security.AxConfig
		if fac != nil {
			secCfg, err = fac.Gen()
			if err != nil {
				s.Fatal("could not generate security config: ", err)
			}
			cfg.RouterConfigParams = append(cfg.RouterConfigParams, secCfg.RouterParams()...)
		}
		testing.ContextLog(ctx, "APPLY RT SETTINGS")
		if err = rt.ApplyRouterSettings(ctx, cfg.RouterConfigParams); err != nil {
			s.Error("Could not set ax settings ", err)
		}
		testing.ContextLog(ctx, "Get RT IP")
		ip, err := rt.GetRouterIP(ctx)
		if err != nil {
			s.Error("Could not get ip addr ", err)
		}

		s.Logf("IP IS %s", ip)
		if fac != nil {
			cfg.DutConnOptions = append(cfg.DutConnOptions, dutcfg.ConnSecurity(secCfg.SecConfig()))
		}

		if err := testing.Poll(ctx, func(ctx context.Context) error {
			_, err = tf.ConnectWifi(ctx, cfg.Ssid, cfg.DutConnOptions...)
			if err != nil {
				if expectedFailure {
					s.Log("Failed to connect to WiFi as expected")
					// If we expect to fail, then this test is already done.
					return nil
				}

				return errors.Wrap(err, "failed to connect to WiFi")
			}
			return nil
		}, &testing.PollOptions{Timeout: 120 * time.Second}); err != nil {
			s.Fatal("Failed to connect to WiFi, err: ", err)
			return
		}

		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := tf.PingFromDUT(ctx, ip); err != nil {
				return errors.Wrap(err, "failed to ping router")
			}
			return nil
		}, &testing.PollOptions{Timeout: 60 * time.Second}); err != nil {
			s.Error("Failed to ping from the DUT ", err)
			return
		}

		defer func(ctx context.Context) {
			if err := tf.CleanDisconnectWifi(ctx); err != nil {
				s.Error("Failed to disconnect WiFi, err: ", err)
			}
		}(ctx)
		ctx, cancel = tf.ReserveForDisconnect(ctx)
		defer cancel()
		if expectedFailure {
			s.Fatal("Expected to fail to connect to WiFi, but it was successful")
		}
		s.Log("Connected")

		ping := func(ctx context.Context) error {
			return tf.PingFromDUT(ctx, ip, pingOps...)
		}

		if err := tf.AssertNoDisconnect(ctx, ping); err != nil {
			s.Fatal("Failed to ping from DUT, err: ", err)
		}
		// TODO(crbug.com/1034875): Assert no deauth detected from the server side.
		// TODO(crbug.com/1034875): Maybe some more check on the WiFi capabilities to
		// verify we really have the settings as expected. (ref: crrev.com/c/1995105)
		s.Log("Deconfiguring")
	}

	testcases := s.Param().([]axSimpleConnectTestcase)
	for i, tc := range testcases {

		subtest := func(ctx context.Context, s *testing.State) {
			testOnce(ctx, s, tc.band, tc.apOpts, tc.secConfFac, tc.pingOps, tc.expectedFailure)
		}
		// subtest(ctx, s)

		if !s.Run(ctx, fmt.Sprintf("Testcase #%d", i), subtest) {
			// Stop if any sub-test failed.
			return
		}
	}
	s.Log("Tearing down")
}
