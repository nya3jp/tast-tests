// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/common/network/ping"
	"chromiumos/tast/common/wifi/security"
	"chromiumos/tast/common/wifi/security/wpa"
	"chromiumos/tast/dut"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/remote/wificell/dutcfg"
	"chromiumos/tast/remote/wificell/router/ax"
	"chromiumos/tast/remote/wificell/router/common/support"
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
		// Removed wificell_func_ax Attr due to router stability issues (b/235887204)
		Attr:        []string{"group:wificell", "wificell_unstable"},
		ServiceDeps: []string{wificell.TFServiceName},
		Vars:        []string{"router", "pcap", "routertype"},
		Params: []testing.Param{
			/* Disabled due to <1% pass rate over 30 days. See b/246820339
			{
				// Verifies that DUT can connect to a broadcasted open 802.11ax on channels 100,104
				Name: "80211ax",
				Val: []axSimpleConnectTestcase{{
					band:             ax.Ghz5,
					apOpts:           []ax.Option{ax.Mode(ax.Mode80211ax), ax.SSID("googleTest0"), ax.Hidden(false), ax.ChanBandwidth(100, ax.Bw80)},
					routerSecConfFac: ax.NewSecOpenConfigParamFac(),
					secConfFac:       base.NewConfigFactory(),
				}, {
					band:             ax.Ghz5,
					apOpts:           []ax.Option{ax.Mode(ax.Mode80211ax), ax.SSID("googleTest1"), ax.Hidden(false), ax.ChanBandwidth(104, ax.Bw80)},
					routerSecConfFac: ax.NewSecOpenConfigParamFac(),
					secConfFac:       base.NewConfigFactory(),
				}},
			}
			*/
			/* Disabled due to <1% pass rate over 30 days. See b/246820339
			{
				// Verifies that DUT can connect to a hidden open 802.11ax network on channel 100, 104
				Name: "80211axhidden",
				Val: []axSimpleConnectTestcase{{
					band:             ax.Ghz5,
					apOpts:           []ax.Option{ax.Mode(ax.Mode80211ax), ax.SSID("googleTest0"), ax.Hidden(true), ax.ChanBandwidth(100, ax.Bw80)},
					routerSecConfFac: ax.NewSecOpenConfigParamFac(),
					secConfFac:       base.NewConfigFactory(),
				}, {
					band:             ax.Ghz5,
					apOpts:           []ax.Option{ax.Mode(ax.Mode80211ax), ax.SSID("googleTest1"), ax.Hidden(true), ax.ChanBandwidth(104, ax.Bw80)},
					routerSecConfFac: ax.NewSecOpenConfigParamFac(),
					secConfFac:       base.NewConfigFactory(),
				}},
			}
			*/
			/* Disabled due to <1% pass rate over 30 days. See b/246820339
			{
				// Verifies that DUT can connect to a broadcasted wpa (AES) 802.11ax network on channel 100, 104
				Name: "80211axwpaaes",
				Val: []axSimpleConnectTestcase{{
					band:             ax.Ghz5,
					apOpts:           []ax.Option{ax.Mode(ax.Mode80211ax), ax.SSID("googleTest0"), ax.Hidden(false), ax.ChanBandwidth(100, ax.Bw80)},
					routerSecConfFac: ax.NewSecWPAConfigParamFac("helloworld", ax.AES),
					secConfFac:       wpa.NewConfigFactory("helloworld", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherCCMP)),
				}, {
					band:             ax.Ghz5,
					apOpts:           []ax.Option{ax.Mode(ax.Mode80211ax), ax.SSID("googleTest1"), ax.Hidden(false), ax.ChanBandwidth(104, ax.Bw80)},
					routerSecConfFac: ax.NewSecWPAConfigParamFac("helloworld", ax.AES),
					secConfFac:       wpa.NewConfigFactory("helloworld", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherCCMP)),
				}},
			}
			*/
			/* Disabled due to <1% pass rate over 30 days. See b/246820339
			{
				// Verifies that DUT can connect to a hidden wpa (AES) 802.11ax network on channel 100, 104
				Name: "80211axwpaaeshidden",
				Val: []axSimpleConnectTestcase{{
					band:             ax.Ghz5,
					apOpts:           []ax.Option{ax.Mode(ax.Mode80211ax), ax.SSID("googleTest0"), ax.Hidden(true), ax.ChanBandwidth(100, ax.Bw40)},
					routerSecConfFac: ax.NewSecWPAConfigParamFac("helloworld", ax.AES),
					secConfFac:       wpa.NewConfigFactory("helloworld", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherCCMP)),
				}, {
					band:             ax.Ghz5,
					apOpts:           []ax.Option{ax.Mode(ax.Mode80211ax), ax.SSID("googleTest1"), ax.Hidden(true), ax.ChanBandwidth(104, ax.Bw40)},
					routerSecConfFac: ax.NewSecWPAConfigParamFac("helloworld", ax.AES),
					secConfFac:       wpa.NewConfigFactory("helloworld", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherCCMP)),
				}},
			}
			*/
			/* Disabled due to <1% pass rate over 30 days. See b/246820339
			{
				// Verifies that DUT can connect to a broadcasted open 802.11ax on channels 100 with 40Mhz channel width on the 5ghz band
				Name: "80211ax40mhz5ghz",
				Val: []axSimpleConnectTestcase{{
					band:             ax.Ghz5,
					apOpts:           []ax.Option{ax.Mode(ax.Mode80211ax), ax.SSID("googleTest0"), ax.Hidden(false), ax.ChanBandwidth(100, ax.Bw40)},
					routerSecConfFac: ax.NewSecOpenConfigParamFac(),
					secConfFac:       base.NewConfigFactory(),
				}},
			}
			*/
			/* Disabled due to <1% pass rate over 30 days. See b/246820339
			{
				// Verifies that DUT can connect to a broadcasted open 802.11ax on channels 100 with 80Mhz channel width on the 5ghz band
				Name: "80211ax80mhz5ghz",
				Val: []axSimpleConnectTestcase{{
					band:             ax.Ghz5,
					apOpts:           []ax.Option{ax.Mode(ax.Mode80211ax), ax.SSID("googleTest0"), ax.Hidden(false), ax.ChanBandwidth(100, ax.Bw80)},
					routerSecConfFac: ax.NewSecOpenConfigParamFac(),
					secConfFac:       base.NewConfigFactory(),
				}},
			}
			*/
			/* Disabled due to <1% pass rate over 30 days. See b/246820339
			{
				// Verifies that DUT can connect to a broadcasted open 802.11ax on channels 100 with 160Mhz channel width on the 5ghz band
				Name: "80211ax160mhz5ghz",
				Val: []axSimpleConnectTestcase{{
					band:             ax.Ghz5,
					apOpts:           []ax.Option{ax.Mode(ax.Mode80211ax), ax.SSID("googleTest0"), ax.Hidden(false), ax.ChanBandwidth(100, ax.Bw160)},
					routerSecConfFac: ax.NewSecOpenConfigParamFac(),
					secConfFac:       base.NewConfigFactory(),
				}},
			}
			*/
			/* Disabled due to <1% pass rate over 30 days. See b/241943857
			{
				// Verifies that DUT can connect to a broadcasted wpa (AES) 802.11ax network on channel 85, 41 on the 6ghz band
				Name: "80211axwpa6ghz",
				Val: []axSimpleConnectTestcase{{
					band:             ax.Ghz6,
					apOpts:           []ax.Option{ax.Mode(ax.Mode80211ax), ax.SSID("googleTest0"), ax.Hidden(false), ax.ChanBandwidth(85, ax.Bw20)},
					routerSecConfFac: ax.NewSecSAEConfigParamFac("helloworld", ax.AES),
					secConfFac:       wpa.NewConfigFactory("helloworld", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherCCMP)),
				}, {
					band:             ax.Ghz6,
					apOpts:           []ax.Option{ax.Mode(ax.Mode80211ax), ax.SSID("googleTest1"), ax.Hidden(false), ax.ChanBandwidth(41, ax.Bw20)},
					routerSecConfFac: ax.NewSecSAEConfigParamFac("helloworld", ax.AES),
					secConfFac:       wpa.NewConfigFactory("helloworld", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherCCMP)),
				}},
				ExtraHardwareDeps: hwdep.D(hwdep.Wifi80211ax6E()),
			}
			*/
			/* Disabled due to <1% pass rate over 30 days. See b/241943857
			{
				// Verifies that DUT can connect to a hidden wpa (AES) 802.11ax network on channel 85, 41 on the 6ghz band
				Name: "80211axwpahidden6ghz",
				Val: []axSimpleConnectTestcase{{
					band:             ax.Ghz6,
					apOpts:           []ax.Option{ax.Mode(ax.Mode80211ax), ax.SSID("googleTest0"), ax.Hidden(true), ax.ChanBandwidth(85, ax.Bw20)},
					routerSecConfFac: ax.NewSecSAEConfigParamFac("helloworld", ax.AES),
					secConfFac:       wpa.NewConfigFactory("helloworld", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherCCMP)),
				}, {
					band:             ax.Ghz6,
					apOpts:           []ax.Option{ax.Mode(ax.Mode80211ax), ax.SSID("googleTest1"), ax.Hidden(true), ax.ChanBandwidth(41, ax.Bw20)},
					routerSecConfFac: ax.NewSecSAEConfigParamFac("helloworld", ax.AES),
					secConfFac:       wpa.NewConfigFactory("helloworld", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherCCMP)),
				}},
				ExtraHardwareDeps: hwdep.D(hwdep.Wifi80211ax6E()),
			}
			*/
			{
				// Verifies that DUT can connect to a broadcasted wpa (AES) 802.11ax with 40Mhz channel width on the 6ghz band
				Name: "80211axwpa40mhz6ghz",
				Val: []axSimpleConnectTestcase{{
					band:             ax.Ghz6,
					apOpts:           []ax.Option{ax.Mode(ax.Mode80211ax), ax.SSID("googleTest0"), ax.Hidden(false), ax.ChanBandwidth(85, ax.Bw40)},
					routerSecConfFac: ax.NewSecSAEConfigParamFac("helloworld", ax.AES),
					secConfFac:       wpa.NewConfigFactory("helloworld", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherCCMP)),
				}},
				ExtraHardwareDeps: hwdep.D(hwdep.Wifi80211ax6E()),
			},
			{
				// Verifies that DUT can connect to a broadcasted wpa (AES) 802.11ax with 80Mhz channel width on the 6ghz band
				Name: "80211axwpa80mhz6ghz",
				Val: []axSimpleConnectTestcase{{
					band:             ax.Ghz6,
					apOpts:           []ax.Option{ax.Mode(ax.Mode80211ax), ax.SSID("googleTest0"), ax.Hidden(false), ax.ChanBandwidth(85, ax.Bw80)},
					routerSecConfFac: ax.NewSecSAEConfigParamFac("helloworld", ax.AES),
					secConfFac:       wpa.NewConfigFactory("helloworld", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherCCMP)),
				}},
				ExtraHardwareDeps: hwdep.D(hwdep.Wifi80211ax6E()),
			},
			{
				// Verifies that DUT can connect to a broadcasted wpa (AES) 802.11ax with 40Mhz channel width on the 6ghz band
				Name: "80211axwpa160mhz6ghz",
				Val: []axSimpleConnectTestcase{{
					band:             ax.Ghz6,
					apOpts:           []ax.Option{ax.Mode(ax.Mode80211ax), ax.SSID("googleTest0"), ax.Hidden(false), ax.ChanBandwidth(85, ax.Bw160)},
					routerSecConfFac: ax.NewSecSAEConfigParamFac("helloworld", ax.AES),
					secConfFac:       wpa.NewConfigFactory("helloworld", wpa.Mode(wpa.ModePureWPA), wpa.Ciphers(wpa.CipherCCMP)),
				}},
				ExtraHardwareDeps: hwdep.D(hwdep.Wifi80211ax6E()),
			},
		},
	})
}

type axSimpleConnectTestcase struct {
	apOpts           []ax.Option
	band             ax.BandEnum
	routerSecConfFac ax.SecConfigParamFac
	secConfFac       security.ConfigFactory
	pingOps          []ping.Option
	expectedFailure  bool
}

func AxSimpleConnect(ctx context.Context, s *testing.State) {
	var tfOps []wificell.TFOption
	router, ok := s.Var("router")
	if !ok || router == "" {
		var err error
		testing.ContextLogf(ctx, "Router name not specified, building default router hostname based on DUT hostname %q", s.DUT().HostName())
		router, err = s.DUT().CompanionDeviceHostname(dut.CompanionSuffixRouter)
		if err != nil {
			s.Fatalf("Failed to synthesize default router name from DUT hostname %q: %v", s.DUT().HostName(), err)
		}
		testing.ContextLogf(ctx, "Using default router name %q for %s router", router, support.AxT.String())
	}
	tfOps = append(tfOps, wificell.TFRouter(router))
	tfOps = append(tfOps, wificell.TFHostUsers(map[string]string{
		router: "admin",
	}))

	// Parse the router's model.
	var axType ax.DeviceType
	routerTypeVar, ok := s.Var("routertype")
	if !ok || len(routerTypeVar) == 0 || strings.EqualFold(routerTypeVar, support.AxT.String()) {
		axType = ax.Unknown
	} else {
		var err error
		axType, err = ax.DeviceTypeFromString(routerTypeVar)
		if err != nil {
			s.Fatalf("Invalid routertype %q: %v", routerTypeVar, err)
		}
	}

	var tfRouterType support.RouterType
	if axType != ax.Unknown {
		tfRouterType = support.AxT
	} else {
		tfRouterType = support.UnknownT
	}
	tfOps = append(tfOps, wificell.TFRouterType(tfRouterType))

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

	if tf.Router().RouterType() != support.AxT {
		s.Fatal("Test fixture router is not an AX router")
	}
	rt, ok := tf.Router().(*ax.Router)
	if !ok {
		s.Fatal("router is not an ax router")
	}
	if axType == ax.Unknown {
		axType, err = rt.ResolveAXDeviceType(ctx)
		if err != nil {
			s.Fatalf("Failed to resolve AX DeviceType from router, err: %v ", err)
		}
	}
	testing.ContextLogf(ctx, "Resolved AX DeviceType to be %q", axType.String())

	// Back up current router configuration.
	backupString, err := rt.RetrieveConfiguration(ctx)
	if err != nil {
		s.Fatal("Could not retrieve current router configuration: ", err)
	}
	backupMap := make(map[string]ax.ConfigParam)
	defer rt.UpdateConfig(ctx, backupMap)
	ctx, cancel = tf.ReserveForClose(ctx)
	defer cancel()
	// subroutine to be run by each subtest.
	testOnce := func(ctx context.Context, s *testing.State, band ax.BandEnum, options []ax.Option, rFac ax.SecConfigParamFac, secFac security.ConfigFactory, pingOps []ping.Option, expectedFailure bool) {
		cfg := ax.Config{
			Band:              ax.BandToRadio(axType, band),
			Type:              axType,
			NVRAMOut:          &backupString,
			RouterRecoveryMap: backupMap}

		// Apply router options
		for _, opt := range options {
			opt(&cfg)
		}

		// Generate security config and update necessary router options
		if rFac != nil {
			cfgParamList, err := rFac.Gen(cfg.Type, band)
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
	defaultOpts := []ax.Option{ax.Radio(true)}
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
