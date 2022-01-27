// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wifi/security/base"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/dutcfg"
	"chromiumos/tast/remote/wificell/router"
	"chromiumos/tast/remote/wificell/router/axrouter"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: AxSimpleConnect,
		Desc: "Verifies that DUT can connect to an AX host via AP in different WiFi configuration",
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
					band:             axrouter.Ghz5,
					apOpts:           []axrouter.Option{axrouter.Mode(axrouter.Mode80211ax), axrouter.SSID("googleTest0"), axrouter.Hidden(false), axrouter.ChanBandwidth(100, axrouter.Bw80)},
					routerSecConfFac: axrouter.NewSecOpenConfigParamFac(),
					secConfFac:       base.NewConfigFactory(),
				}, {
					band:             axrouter.Ghz5,
					apOpts:           []axrouter.Option{axrouter.Mode(axrouter.Mode80211ax), axrouter.SSID("googleTest1"), axrouter.Hidden(false), axrouter.ChanBandwidth(104, axrouter.Bw80)},
					routerSecConfFac: axrouter.NewSecOpenConfigParamFac(),
					secConfFac:       base.NewConfigFactory(),
				}},
			}, {
				// Verifies that DUT can connect to a hidden 802.11ax network on channel 100, 104
				Name: "80211axopenhidden",
				Val: []axSimpleConnectTestcase{{
					band:             axrouter.Ghz5,
					apOpts:           []axrouter.Option{axrouter.Mode(axrouter.Mode80211ax), axrouter.SSID("googleTest0"), axrouter.Hidden(true), axrouter.ChanBandwidth(100, axrouter.Bw80)},
					routerSecConfFac: axrouter.NewSecOpenConfigParamFac(),
					secConfFac:       base.NewConfigFactory(),
				}, {
					band:             axrouter.Ghz5,
					apOpts:           []axrouter.Option{axrouter.Mode(axrouter.Mode80211ax), axrouter.SSID("googleTest1"), axrouter.Hidden(true), axrouter.ChanBandwidth(104, axrouter.Bw80)},
					routerSecConfFac: axrouter.NewSecOpenConfigParamFac(),
					secConfFac:       base.NewConfigFactory(),
				}},
			},
			{
				// Verifies that DUT can connect to an open wpa (AES) 802.11ax network on channel 100, 104
				Name: "80211axwpaaes",
				Val: []axSimpleConnectTestcase{{
					band:             axrouter.Ghz5,
					apOpts:           []axrouter.Option{axrouter.Mode(axrouter.Mode80211ax), axrouter.SSID("googleTest0"), axrouter.Hidden(false), axrouter.ChanBandwidth(100, axrouter.Bw80)},
					routerSecConfFac: axrouter.NewSecWPAConfigParamFac("helloworld", axrouter.AES),
					secConfFac:       wpa.NewConfigFactory("helloworld", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherCCMP)),
				}, {
					band:             axrouter.Ghz5,
					apOpts:           []axrouter.Option{axrouter.Mode(axrouter.Mode80211ax), axrouter.SSID("googleTest1"), axrouter.Hidden(false), axrouter.ChanBandwidth(104, axrouter.Bw80)},
					routerSecConfFac: axrouter.NewSecWPAConfigParamFac("helloworld", axrouter.AES),
					secConfFac:       wpa.NewConfigFactory("helloworld", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherCCMP)),
				}},
			},
			{
				// Verifies that DUT can connect to a hidden (AES) 802.11ax network on channel 100, 104
				Name: "80211axwpahiddenaes",
				Val: []axSimpleConnectTestcase{{
					band:             axrouter.Ghz5,
					apOpts:           []axrouter.Option{axrouter.Mode(axrouter.Mode80211ax), axrouter.SSID("googleTest0"), axrouter.Hidden(true), axrouter.ChanBandwidth(100, axrouter.Bw40)},
					routerSecConfFac: axrouter.NewSecWPAConfigParamFac("helloworld", axrouter.AES),
					secConfFac:       wpa.NewConfigFactory("helloworld", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherCCMP)),
				}, {
					band:             axrouter.Ghz5,
					apOpts:           []axrouter.Option{axrouter.Mode(axrouter.Mode80211ax), axrouter.SSID("googleTest1"), axrouter.Hidden(true), axrouter.ChanBandwidth(104, axrouter.Bw40)},
					routerSecConfFac: axrouter.NewSecWPAConfigParamFac("helloworld", axrouter.AES),
					secConfFac:       wpa.NewConfigFactory("helloworld", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherCCMP)),
				}},
			},
			{
				// Verifies that DUT can connect to an open 802.11ax on channels 100 with 40Mhz channel width on the 5ghz band
				Name: "80211axopen40mhz5ghz",
				Val: []axSimpleConnectTestcase{{
					band:             axrouter.Ghz5,
					apOpts:           []axrouter.Option{axrouter.Mode(axrouter.Mode80211ax), axrouter.SSID("googleTest0"), axrouter.Hidden(false), axrouter.ChanBandwidth(100, axrouter.Bw40)},
					routerSecConfFac: axrouter.NewSecOpenConfigParamFac(),
					secConfFac:       base.NewConfigFactory(),
				}},
			},
			{
				// Verifies that DUT can connect to an open 802.11ax on channels 100 with 80Mhz channel width on the 5ghz band
				Name: "80211axopen80mhz5ghz",
				Val: []axSimpleConnectTestcase{{
					band:             axrouter.Ghz5,
					apOpts:           []axrouter.Option{axrouter.Mode(axrouter.Mode80211ax), axrouter.SSID("googleTest0"), axrouter.Hidden(false), axrouter.ChanBandwidth(100, axrouter.Bw80)},
					routerSecConfFac: axrouter.NewSecOpenConfigParamFac(),
					secConfFac:       base.NewConfigFactory(),
				}},
			},
			{
				// Verifies that DUT can connect to an open 802.11ax on channels 100 with 160Mhz channel width on the 5ghz band
				Name: "80211axopen160mhz5ghz",
				Val: []axSimpleConnectTestcase{{
					band:             axrouter.Ghz5,
					apOpts:           []axrouter.Option{axrouter.Mode(axrouter.Mode80211ax), axrouter.SSID("googleTest0"), axrouter.Hidden(false), axrouter.ChanBandwidth(100, axrouter.Bw160)},
					routerSecConfFac: axrouter.NewSecOpenConfigParamFac(),
					secConfFac:       base.NewConfigFactory(),
				}},
			},
			{
				// Verifies that DUT can connect to an open wpa (AES) 802.11ay network on channel 85, 41
				Name: "80211axwpaopen6ghz",
				Val: []axSimpleConnectTestcase{{
					band:             axrouter.Ghz6,
					apOpts:           []axrouter.Option{axrouter.Mode(axrouter.Mode80211ax), axrouter.SSID("googleTest0"), axrouter.Hidden(false), axrouter.ChanBandwidth(85, axrouter.Bw20)},
					routerSecConfFac: axrouter.NewSecSAEConfigParamFac("helloworld", axrouter.AES),
					secConfFac:       wpa.NewConfigFactory("helloworld", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherCCMP)),
				}, {
					band:             axrouter.Ghz6,
					apOpts:           []axrouter.Option{axrouter.Mode(axrouter.Mode80211ax), axrouter.SSID("googleTest1"), axrouter.Hidden(false), axrouter.ChanBandwidth(41, axrouter.Bw20)},
					routerSecConfFac: axrouter.NewSecSAEConfigParamFac("helloworld", axrouter.AES),
					secConfFac:       wpa.NewConfigFactory("helloworld", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherCCMP)),
				}},
				ExtraHardwareDeps: hwdep.D(hwdep.Wifi80211ax6E()),
			},
			{
				// Verifies that DUT can connect to a hidden wpa (AES) 802.11ay network on channel 85, 41
				Name: "80211axwpahidden6ghz",
				Val: []axSimpleConnectTestcase{{
					band:             axrouter.Ghz6,
					apOpts:           []axrouter.Option{axrouter.Mode(axrouter.Mode80211ax), axrouter.SSID("googleTest0"), axrouter.Hidden(true), axrouter.ChanBandwidth(85, axrouter.Bw20)},
					routerSecConfFac: axrouter.NewSecSAEConfigParamFac("helloworld", axrouter.AES),
					secConfFac:       wpa.NewConfigFactory("helloworld", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherCCMP)),
				}, {
					band:             axrouter.Ghz6,
					apOpts:           []axrouter.Option{axrouter.Mode(axrouter.Mode80211ax), axrouter.SSID("googleTest1"), axrouter.Hidden(true), axrouter.ChanBandwidth(41, axrouter.Bw20)},
					routerSecConfFac: axrouter.NewSecSAEConfigParamFac("helloworld", axrouter.AES),
					secConfFac:       wpa.NewConfigFactory("helloworld", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherCCMP)),
				}},
				ExtraHardwareDeps: hwdep.D(hwdep.Wifi80211ax6E()),
			},
			{
				// Verifies that DUT can connect to an open wpa (AES) 802.11ay with 40Mhz channel width on the 6ghz band
				Name: "80211wpaopen40mhz6ghz",
				Val: []axSimpleConnectTestcase{{
					band:             axrouter.Ghz6,
					apOpts:           []axrouter.Option{axrouter.Mode(axrouter.Mode80211ax), axrouter.SSID("googleTest0"), axrouter.Hidden(false), axrouter.ChanBandwidth(85, axrouter.Bw40)},
					routerSecConfFac: axrouter.NewSecSAEConfigParamFac("helloworld", axrouter.AES),
					secConfFac:       wpa.NewConfigFactory("helloworld", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherCCMP)),
				}},
				ExtraHardwareDeps: hwdep.D(hwdep.Wifi80211ax6E()),
			},
			{
				// Verifies that DUT can connect to an open wpa (AES) 802.11ay with 80Mhz channel width on the 6ghz band
				Name: "80211wpaopen80mhz6ghz",
				Val: []axSimpleConnectTestcase{{
					band:             axrouter.Ghz6,
					apOpts:           []axrouter.Option{axrouter.Mode(axrouter.Mode80211ax), axrouter.SSID("googleTest0"), axrouter.Hidden(false), axrouter.ChanBandwidth(85, axrouter.Bw80)},
					routerSecConfFac: axrouter.NewSecSAEConfigParamFac("helloworld", axrouter.AES),
					secConfFac:       wpa.NewConfigFactory("helloworld", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherCCMP)),
				}},
				ExtraHardwareDeps: hwdep.D(hwdep.Wifi80211ax6E()),
			},
			{
				// Verifies that DUT can connect to an open wpa (AES) 802.11ay with 40Mhz channel width on the 6ghz band
				Name: "80211wpaopen160mhz6ghz",
				Val: []axSimpleConnectTestcase{{
					band:             axrouter.Ghz6,
					apOpts:           []axrouter.Option{axrouter.Mode(axrouter.Mode80211ax), axrouter.SSID("googleTest0"), axrouter.Hidden(false), axrouter.ChanBandwidth(85, axrouter.Bw160)},
					routerSecConfFac: axrouter.NewSecSAEConfigParamFac("helloworld", axrouter.AES),
					secConfFac:       wpa.NewConfigFactory("helloworld", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherCCMP)),
				}},
				ExtraHardwareDeps: hwdep.D(hwdep.Wifi80211ax6E()),
			},
		},
	})
}

type axSimpleConnectTestcase struct {
	apOpts           []axrouter.Option
	band             axrouter.BandEnum
	routerSecConfFac axrouter.SecConfigParamFac
	secConfFac       security.ConfigFactory
	pingOps          []ping.Option
	expectedFailure  bool
}

func AxSimpleConnect(ctx context.Context, s *testing.State) {
	var tfOps []wificell.TFOption
	if router, ok := s.Var("router"); ok && router != "" {
		tfOps = append(tfOps, wificell.TFRouter(router))
	}

	// Parse the router's model.
	var axType = axrouter.Invalid
	if routertype, ok := s.Var("routertype"); ok && routertype != "" {
		if routertype == "gtax11000" {
			axType = axrouter.GtAx11000
		} else if routertype == "ax6100" {
			axType = axrouter.Ax6100
		} else if routertype == "gtaxe11000" {
			axType = axrouter.GtAxe11000
		}
		testing.ContextLog(ctx, "test running for ", routertype)
	}
	if axType == axrouter.Invalid {
		s.Fatal("AxRouterType not defined. Please specify router type with --routertype (gtax11000|gtaxe11000|ax6100)")
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

	// Back up current router configuration.
	backupString, err := rt.RetrieveConfiguration(ctx)
	if err != nil {
		s.Fatal("Could not retrieve current router configuration: ", err)
	}
	backupMap := make(map[string]axrouter.ConfigParam)
	defer rt.UpdateConfig(ctx, backupMap)
	ctx, cancel = tf.ReserveForClose(ctx)
	defer cancel()
	// subroutine to be run by each subtest.
	testOnce := func(ctx context.Context, s *testing.State, band axrouter.BandEnum, options []axrouter.Option, rFac axrouter.SecConfigParamFac, secFac security.ConfigFactory, pingOps []ping.Option, expectedFailure bool) {
		cfg := axrouter.Config{Band: axrouter.BandToRadio(axType, band),
			Type:              axType,
			NVRAMOut:          &backupString,
			RouterRecoveryMap: backupMap}

		// Apply router options
		for _, opt := range options {
			opt(&cfg)
		}

		// Generate security config and update necessary router options
		if rFac != nil {
			cfgParamList, err := rFac.Gen(axType, band)
			if err != nil {
				s.Fatal("Could not generate security ConfigParam list: ", err)
			}
			cfg.RouterConfigParams = append(cfg.RouterConfigParams, cfgParamList...)
		}
		// Update DUT's connection options.
		if secFac != nil {
			secCfg, err := secFac.Gen()
			if err != nil {
				s.Fatal("Could not generate security factory: ", err)
			}
			cfg.DUTConnOptions = append(cfg.DUTConnOptions, dutcfg.ConnSecurity(secCfg))
		}

		// Update the router
		if err = rt.ApplyRouterSettings(ctx, &cfg); err != nil {
			s.Errorf("Could not apply the desired ax settings %s to the router: %v", cfg.RouterConfigParams, err)
		}

		// Get the router's IP to be used for ping test.
		ip, err := rt.RouterIP(ctx)
		if err != nil {
			s.Error("Could not get the router's IP address: ", err)
		}

		s.Logf("The router's IP is %s", ip)
		// Attempt to discovery and connect to AP.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			_, err = tf.ConnectWifi(ctx, cfg.SSID, cfg.DUTConnOptions...)
			if err != nil {
				if expectedFailure {
					s.Log("Failed to connect to WiFi as expected")
					// If we expect to fail, then this test is already done.
					return nil
				}

				return err
			}
			return nil
		}, &testing.PollOptions{Timeout: 120 * time.Second}); err != nil {
			s.Fatal("Failed to connect to WiFi, err: ", err)
			return
		}

		// Attempt to ping the router.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if err := tf.PingFromDUT(ctx, ip); err != nil {
				return err
			}
			return nil
		}, &testing.PollOptions{Timeout: 60 * time.Second}); err != nil {
			s.Error("Failed to ping from the DUT: ", err)
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

	// Default AP options run on every test,
	defaultOpts := []axrouter.Option{axrouter.Radio(true)}
	for i, tc := range testcases {

		subtest := func(ctx context.Context, s *testing.State) {
			testOnce(ctx, s, tc.band, append(tc.apOpts, defaultOpts...), tc.routerSecConfFac, tc.secConfFac, tc.pingOps, tc.expectedFailure)
		}

		if !s.Run(ctx, fmt.Sprintf("Testcase #%d", i), subtest) {
			// Stop if any sub-test failed.
			return
		}
	}
	s.Log("Tearing down")
}
