// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package spera

import (
	"context"
	"time"

	"chromiumos/tast/common/cros/ui/setup"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/audio/crastestclient"
	et "chromiumos/tast/local/bundles/cros/spera/everydaymultitaskingcuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/cuj/bluetooth"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type multiTaskingParam struct {
	tier        cuj.Tier
	appName     string
	enableBT    bool // enable the bluetooth or not
	browserType browser.Type
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         EverydayMultiTaskingCUJ,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures the performance of everyday multi-tasking CUJ",
		Contacts:     []string{"xliu@cienet.com", "jane.yang@cienet.com"},
		SoftwareDeps: []string{"chrome", "arc"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Vars: []string{
			"spera.cuj_mute",      // Optional. Mute the DUT during the test.
			"spera.cuj_mode",      // Optional. Expecting "tablet" or "clamshell".
			"spera.collectTrace",  // Optional. Expecting "enable" or "disable", default is "disable".
			"spera.bt_devicename", // Required for Bluetooth subtests.
		},
		Data: []string{"cca_ui.js", cujrecorder.SystemTraceConfigFile},
		Params: []testing.Param{
			{
				Name:    "basic_ytmusic",
				Fixture: "loggedInAndKeepState",
				Timeout: 20 * time.Minute,
				Val: multiTaskingParam{
					tier:     cuj.Basic,
					appName:  et.YoutubeMusicAppName,
					enableBT: false,
				},
			}, {
				Name:              "basic_lacros_ytmusic",
				Fixture:           "loggedInAndKeepStateLacros",
				Timeout:           20 * time.Minute,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: multiTaskingParam{
					tier:        cuj.Basic,
					appName:     et.YoutubeMusicAppName,
					enableBT:    false,
					browserType: browser.TypeLacros,
				},
			}, {
				Name:              "basic_ytmusic_crosbolt",
				Fixture:           "loggedInAndKeepState",
				Timeout:           20 * time.Minute,
				ExtraAttr:         []string{"group:crosbolt", "crosbolt_perbuild"},
				ExtraHardwareDeps: hwdep.D(setup.PerfCUJDevices()),
				Val: multiTaskingParam{
					tier:     cuj.Basic,
					appName:  et.YoutubeMusicAppName,
					enableBT: false,
				},
			}, {
				Name:    "basic_ytmusic_bluetooth",
				Fixture: "loggedInAndKeepState",
				Timeout: 20 * time.Minute,
				Val: multiTaskingParam{
					tier:     cuj.Basic,
					appName:  et.YoutubeMusicAppName,
					enableBT: true,
				},
			}, {
				Name:              "basic_lacros_ytmusic_bluetooth",
				Fixture:           "loggedInAndKeepStateLacros",
				Timeout:           20 * time.Minute,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: multiTaskingParam{
					tier:        cuj.Basic,
					appName:     et.YoutubeMusicAppName,
					enableBT:    true,
					browserType: browser.TypeLacros,
				},
			}, {
				Name:    "basic_spotify_bluetooth",
				Fixture: "loggedInAndKeepState",
				Timeout: 20 * time.Minute,
				Val: multiTaskingParam{
					tier:     cuj.Basic,
					appName:  et.SpotifyAppName,
					enableBT: true,
				},
			}, {
				Name:              "basic_lacros_spotify_bluetooth",
				Fixture:           "loggedInAndKeepStateLacros",
				Timeout:           20 * time.Minute,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: multiTaskingParam{
					tier:        cuj.Basic,
					appName:     et.SpotifyAppName,
					enableBT:    true,
					browserType: browser.TypeLacros,
				},
			}, {
				Name:              "plus_helloworld",
				Fixture:           "loggedInAndKeepState",
				Timeout:           15 * time.Minute,
				ExtraAttr:         []string{"group:crosbolt", "crosbolt_perbuild"},
				ExtraSoftwareDeps: []string{"android_p"},
				Val: multiTaskingParam{
					tier:     cuj.Plus,
					appName:  et.HelloWorldAppName,
					enableBT: false,
				},
			}, {
				Name:              "plus_helloworld_vm",
				Fixture:           "loggedInAndKeepState",
				Timeout:           15 * time.Minute,
				ExtraAttr:         []string{"group:crosbolt", "crosbolt_perbuild"},
				ExtraSoftwareDeps: []string{"android_vm"},
				Val: multiTaskingParam{
					tier:     cuj.Plus,
					appName:  et.HelloWorldAppName,
					enableBT: false,
				},
			}, {
				Name:    "plus_ytmusic",
				Fixture: "loggedInAndKeepState",
				Timeout: 30 * time.Minute,
				Val: multiTaskingParam{
					tier:     cuj.Plus,
					appName:  et.YoutubeMusicAppName,
					enableBT: false,
				},
			}, {
				Name:              "plus_lacros_ytmusic",
				Fixture:           "loggedInAndKeepStateLacros",
				Timeout:           30 * time.Minute,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: multiTaskingParam{
					tier:        cuj.Plus,
					appName:     et.YoutubeMusicAppName,
					enableBT:    false,
					browserType: browser.TypeLacros,
				},
			}, {
				Name:              "plus_ytmusic_crosbolt",
				Fixture:           "loggedInAndKeepState",
				Timeout:           30 * time.Minute,
				ExtraAttr:         []string{"group:crosbolt", "crosbolt_perbuild"},
				ExtraHardwareDeps: hwdep.D(setup.PerfCUJDevices()),
				Val: multiTaskingParam{
					tier:     cuj.Plus,
					appName:  et.YoutubeMusicAppName,
					enableBT: false,
				},
			}, {
				Name:    "plus_ytmusic_bluetooth",
				Fixture: "loggedInAndKeepState",
				Timeout: 30 * time.Minute,
				Val: multiTaskingParam{
					tier:     cuj.Plus,
					appName:  et.YoutubeMusicAppName,
					enableBT: true,
				},
			}, {
				Name:              "plus_lacros_ytmusic_bluetooth",
				Fixture:           "loggedInAndKeepStateLacros",
				Timeout:           30 * time.Minute,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: multiTaskingParam{
					tier:        cuj.Plus,
					appName:     et.YoutubeMusicAppName,
					enableBT:    true,
					browserType: browser.TypeLacros,
				},
			}, {
				Name:    "plus_spotify_bluetooth",
				Fixture: "loggedInAndKeepState",
				Timeout: 30 * time.Minute,
				Val: multiTaskingParam{
					tier:     cuj.Plus,
					appName:  et.SpotifyAppName,
					enableBT: true,
				},
			}, {
				Name:              "plus_lacros_spotify_bluetooth",
				Fixture:           "loggedInAndKeepStateLacros",
				Timeout:           30 * time.Minute,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: multiTaskingParam{
					tier:        cuj.Plus,
					appName:     et.SpotifyAppName,
					enableBT:    true,
					browserType: browser.TypeLacros,
				},
			}, {
				Name:    "plus_spotify_quickcheck",
				Fixture: "loggedInAndKeepState",
				Timeout: 15 * time.Minute,
				Val: multiTaskingParam{
					tier:     cuj.Plus,
					appName:  et.SpotifyAppName,
					enableBT: false,
				},
			},
		},
	})
}

func EverydayMultiTaskingCUJ(ctx context.Context, s *testing.State) {
	param := s.Param().(multiTaskingParam)
	tier := param.tier
	app := param.appName
	enableBT := param.enableBT

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	a := s.FixtValue().(cuj.FixtureData).ARC

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	if _, ok := s.Var("spera.cuj_mute"); ok {
		if err := crastestclient.Mute(ctx); err != nil {
			s.Fatal("Failed to mute audio: ", err)
		}
		defer crastestclient.Unmute(cleanupCtx)
	}

	isBtEnabled, err := bluetooth.IsEnabled(ctx)
	if err != nil {
		s.Fatal("Failed to get bluetooth status: ", err)
	}

	if enableBT {
		testing.ContextLog(ctx, "Start to connect bluetooth")
		deviceName := s.RequiredVar("spera.bt_devicename")
		if err := bluetooth.ConnectDevice(ctx, deviceName); err != nil {
			s.Fatal("Failed to connect bluetooth: ", err)
		}
		if !isBtEnabled {
			defer func(ctx context.Context) {
				if err := bluetooth.Disable(ctx); err != nil {
					s.Fatal("Failed to disable bluetooth: ", err)
				}
			}(cleanupCtx)
		}
	} else if isBtEnabled {
		testing.ContextLog(ctx, "Start to disable bluetooth")
		if err := bluetooth.Disable(ctx); err != nil {
			s.Fatal("Failed to disable bluetooth: ", err)
		}
		defer func(ctx context.Context) {
			if err := bluetooth.Enable(ctx); err != nil {
				s.Fatal("Failed to connect bluetooth: ", err)
			}
		}(cleanupCtx)
	}

	// The cras active node will change if the bluetooth has been connected or disconnected.
	// Wait until cras node change completes before doing output volume testing.
	condition := func(cn *audio.CrasNode) bool {
		if !cn.Active {
			return false
		}

		// According to src/third_party/adhd/cras/README.dbus-api,
		// the audio.CrasNode.Type should be "BLUETOOTH" when a BlueTooth device
		// is used as input or output.
		if enableBT {
			return cn.Type == "BLUETOOTH"
		}
		return cn.Type != "BLUETOOTH"
	}
	cras, err := audio.NewCras(ctx)
	if err != nil {
		s.Fatal("Failed to create cras: ", err)
	}
	if err := cras.WaitForDeviceUntil(ctx, condition, 40*time.Second); err != nil {
		s.Fatalf("Failed to wait for cras nodes to be in expected status: %v; please check the DUT log for possible audio device connection issue", err)
	}

	// Spotify login account.
	var account string
	if app == et.SpotifyAppName {
		account = cr.Creds().User
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	var tabletMode bool
	if mode, ok := s.Var("spera.cuj_mode"); ok {
		tabletMode = mode == "tablet"
		cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, tabletMode)
		if err != nil {
			s.Fatalf("Failed to enable tablet mode to %v: %v", tabletMode, err)
		}
		defer cleanup(cleanupCtx)
	} else {
		// Use default screen mode of the DUT.
		tabletMode, err = ash.TabletModeEnabled(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to get DUT default screen mode: ", err)
		}
	}
	s.Log("Running test with tablet mode: ", tabletMode)
	if tabletMode {
		cleanup, err := display.RotateToLandscape(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to rotate display to landscape: ", err)
		}
		defer cleanup(cleanupCtx)
	}
	traceConfigPath := ""
	if collect, ok := s.Var("spera.collectTrace"); ok && collect == "enable" {
		traceConfigPath = s.DataPath(cujrecorder.SystemTraceConfigFile)
	}
	ccaScriptPaths := []string{s.DataPath("cca_ui.js")}
	testRunParams := et.NewRunParams(tier, ccaScriptPaths, s.OutDir(), app, account, traceConfigPath, tabletMode, enableBT)
	if err := et.Run(ctx, cr, param.browserType, a, testRunParams); err != nil {
		s.Fatal("Failed to run everyday multi-tasking cuj test: ", err)
	}
}
