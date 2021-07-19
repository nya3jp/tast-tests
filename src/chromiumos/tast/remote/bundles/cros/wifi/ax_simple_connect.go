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
			"billyzhao@google.com",
			"chromeos-wifi-champs@google.com", // WiFi oncall rotation; or http://b/new?component=893827
		},
		Attr:        []string{},
		ServiceDeps: []string{wificell.TFServiceName},
		Vars:        []string{"router", "pcap", "routertype"},
		Params: []testing.Param{
			{
				// Verifies that DUT can connect to an open 802.11ax on channels 100,104
				Name: "80211axopen",
				Val: []axSimpleConnectTestcase{{
					band:       router.Wl2,
					apOpts:     []router.Option{router.Mode(router.Mode80211ax), router.Ssid("googleTest0"), router.Hidden(false), router.ChanBandwidth(100, router.Bw80)},
					secConfFac: axbase.NewConfigFactory(router.Wl2),
				}, {
					band:       router.Wl2,
					apOpts:     []router.Option{router.Mode(router.Mode80211ax), router.Ssid("googleTest1"), router.Hidden(false), router.ChanBandwidth(104, router.Bw80)},
					secConfFac: axbase.NewConfigFactory(router.Wl2),
				}},
			}, {
				// Verifies that DUT can connect to a hidden 802.11ax network on channel 100, 104
				Name: "80211axopenhidden",
				Val: []axSimpleConnectTestcase{{
					band:       router.Wl2,
					apOpts:     []router.Option{router.Mode(router.Mode80211ax), router.Ssid("googleTest0"), router.Hidden(true), router.ChanBandwidth(100, router.Bw80)},
					secConfFac: axbase.NewConfigFactory(router.Wl1),
				}, {
					band:       router.Wl2,
					apOpts:     []router.Option{router.Mode(router.Mode80211ax), router.Ssid("googleTest1"), router.Hidden(true), router.ChanBandwidth(104, router.Bw80)},
					secConfFac: axbase.NewConfigFactory(router.Wl1),
				}},
			},
			{
				// Verifies that DUT can connect to an open wpa (AES) 802.11ax network on channel 100, 104
				Name: "80211axwpaaes",
				Val: []axSimpleConnectTestcase{{
					band:       router.Wl2,
					apOpts:     []router.Option{router.Mode(router.Mode80211ax), router.Ssid("googleTest0"), router.Hidden(false), router.ChanBandwidth(100, router.Bw80)},
					secConfFac: axwpa.NewConfigFactory(router.Wl1, "helloworld", router.AES, wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherCCMP)),
				}, {
					band:       router.Wl2,
					apOpts:     []router.Option{router.Mode(router.Mode80211ax), router.Ssid("googleTest1"), router.Hidden(false), router.ChanBandwidth(104, router.Bw80)},
					secConfFac: axwpa.NewConfigFactory(router.Wl1, "helloworld", router.AES, wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherCCMP)),
				}},
			},
			{
				// Verifies that DUT can connect to an open wpa (AES+TKIP) 802.11ax network on channel 100, 104
				Name: "80211axwpamixed",
				Val: []axSimpleConnectTestcase{{
					band:       router.Wl2,
					apOpts:     []router.Option{router.Mode(router.Mode80211ax), router.Ssid("googleTest0"), router.Hidden(true), router.ChanBandwidth(100, router.Bw80)},
					secConfFac: axwpa.NewConfigFactory(router.Wl1, "helloworld", router.TKIPAES, wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherTKIP, wpa.CipherCCMP)),
				}, {
					band:       router.Wl2,
					apOpts:     []router.Option{router.Mode(router.Mode80211ax), router.Ssid("googleTest1"), router.Hidden(true), router.ChanBandwidth(104, router.Bw80)},
					secConfFac: axwpa.NewConfigFactory(router.Wl1, "helloworld", router.TKIPAES, wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherTKIP, wpa.CipherCCMP)),
				}},
			},
			{
				// Verifies that DUT can connect to a hidden (AES) 802.11ax network on channel 100, 104
				Name: "80211axwpahiddenaes",
				Val: []axSimpleConnectTestcase{{
					band:       router.Wl2,
					apOpts:     []router.Option{router.Mode(router.Mode80211ax), router.Ssid("googleTest0"), router.Hidden(true), router.ChanBandwidth(100, router.Bw80)},
					secConfFac: axwpa.NewConfigFactory(router.Wl1, "helloworld", router.AES, wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherCCMP)),
				}, {
					band:       router.Wl2,
					apOpts:     []router.Option{router.Mode(router.Mode80211ax), router.Ssid("googleTest1"), router.Hidden(true), router.ChanBandwidth(104, router.Bw80)},
					secConfFac: axwpa.NewConfigFactory(router.Wl1, "helloworld", router.AES, wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherCCMP)),
				}},
			},
			{
				// Verifies that DUT can connect to a hidden (AES+TKIP) 802.11ax network on channel 100, 104
				Name: "80211axwpahiddenmixed",
				Val: []axSimpleConnectTestcase{{
					band:       router.Wl2,
					apOpts:     []router.Option{router.Mode(router.Mode80211ax), router.Ssid("googleTest0"), router.Hidden(true), router.ChanBandwidth(100, router.Bw80)},
					secConfFac: axwpa.NewConfigFactory(router.Wl1, "helloworld", router.TKIPAES, wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherTKIP, wpa.CipherCCMP)),
				}, {
					band:       router.Wl2,
					apOpts:     []router.Option{router.Mode(router.Mode80211ax), router.Ssid("googleTest1"), router.Hidden(true), router.ChanBandwidth(104, router.Bw80)},
					secConfFac: axwpa.NewConfigFactory(router.Wl1, "helloworld", router.TKIPAES, wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherTKIP, wpa.CipherCCMP)),
				}},
			},
			{
				// Verifies that DUT can connect to an open 802.11ax on channels 100 with 40Mhz channel width on the 5ghz band
				Name: "80211axopen40mhz5ghz",
				Val: []axSimpleConnectTestcase{{
					band:       router.Wl1,
					apOpts:     []router.Option{router.Mode(router.Mode80211ax), router.Ssid("googleTest0"), router.Hidden(false), router.ChanBandwidth(100, router.Bw40)},
					secConfFac: axbase.NewConfigFactory(router.Wl1),
				}},
			},
			{
				// Verifies that DUT can connect to an open 802.11ax on channels 100 with 80Mhz channel width on the 5ghz band
				Name: "80211axopen80mhz5ghz",
				Val: []axSimpleConnectTestcase{{
					band:       router.Wl1,
					apOpts:     []router.Option{router.Mode(router.Mode80211ax), router.Ssid("googleTest0"), router.Hidden(false), router.ChanBandwidth(100, router.Bw80)},
					secConfFac: axbase.NewConfigFactory(router.Wl1),
				}},
			},
			{
				// Verifies that DUT can connect to an open 802.11ax on channels 100 with 160Mhz channel width on the 5ghz band
				Name: "80211axopen160mhz5ghz",
				Val: []axSimpleConnectTestcase{{
					band:       router.Wl1,
					apOpts:     []router.Option{router.Mode(router.Mode80211ax), router.Ssid("googleTest0"), router.Hidden(false), router.ChanBandwidth(100, router.Bw160)},
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
	var axType = router.Invalid
	if routertype, ok := s.Var("routertype"); ok && routertype != "" {
		if routertype == "gtax11000" {
			axType = router.GtAx11000
			testing.ContextLog(ctx, "test running for GtAx11000")
		} else if routertype == "ax6100" {
			axType = router.Ax6100
			testing.ContextLog(ctx, "test running for Ax6100")
		}
	}
	if axType == router.Invalid {
		s.Fatal("AxRouterType not defined. Please specify router type with --routertype (gtax11000|ax6100)")
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

	// Backup current router configuration
	if err := rt.SaveConfiguration(ctx); err != nil {
		s.Fatalf("Could not save current router configuration: ", err)
	}
	//defer rt.RestoreConfiguration(ctx)

	// subroutine to be run by each subtest.
	testOnce := func(ctx context.Context, s *testing.State, band router.BandEnum, options []router.Option, fac security.AxConfigFactory, pingOps []ping.Option, expectedFailure bool) {
		var cfg router.Config
		cfg.Band = band
		cfg.Type = axType
		// Apply router options
		for _, opt := range options {
			opt(&cfg)
		}

		// Generate security config and update necessary router options
		var secCfg security.AxConfig
		if fac != nil {
			secCfg, err = fac.Gen()
			if err != nil {
				s.Fatal("Could not generate security config: ", err)
			}
			cfg.RouterConfigParams = append(cfg.RouterConfigParams, secCfg.RouterParams()...)
		}

		testing.ContextLog(ctx, cfg.RouterConfigParams)
		// Update the router
		if err = rt.ApplyRouterSettings(ctx, cfg.RouterConfigParams); err != nil {
			s.Error("Could not set ax settings: ", err)
		}
		// Give router time to disassociate as the restart command is asynchronous.
		testing.Sleep(ctx, 10*time.Second)
		// Get the router's IP to be used for ping test.
		ip, err := rt.GetRouterIP(ctx)
		if err != nil {
			s.Error("Could not get ip addr: ", err)
		}

		s.Logf("IP IS %s", ip)

		// Update DUT's connection options.
		if fac != nil {
			cfg.DutConnOptions = append(cfg.DutConnOptions, dutcfg.ConnSecurity(secCfg.SecConfig()))
		}

		// Attempt to discovery and connect to AP.
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

		// Attempt to ping the router.
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
		s.Log("Deconfiguring")
	}

	testcases := s.Param().([]axSimpleConnectTestcase)
	for i, tc := range testcases {

		subtest := func(ctx context.Context, s *testing.State) {
			testOnce(ctx, s, tc.band, tc.apOpts, tc.secConfFac, tc.pingOps, tc.expectedFailure)
		}

		if !s.Run(ctx, fmt.Sprintf("Testcase #%d", i), subtest) {
			// Stop if any sub-test failed.
			return
		}
	}
	s.Log("Tearing down")
}
