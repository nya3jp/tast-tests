// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package spera

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/spera/imageeditingcuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const testImage = "Lenna.png"

func init() {
	testing.AddTest(&testing.Test{
		Func:         EDUImageEditingCUJ,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures the performance of editing photos on the web",
		Contacts:     []string{"xliu@cienet.com", "alston.huang@cienet.com", "cienet-development@googlegroups.com"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Data:         []string{testImage, cujrecorder.SystemTraceConfigFile},
		Vars: []string{
			"spera.cuj_mode",     // Optional. Expecting "tablet" or "clamshell".
			"spera.collectTrace", // Optional. Expecting "enable" or "disable", default is "disable".
		},
		Params: []testing.Param{
			{
				Name:    "basic_google_photos",
				Fixture: "enrolledLoggedInToCUJUser",
				Timeout: 5 * time.Minute,
				Val:     browser.TypeAsh,
			},
			{
				Name:              "basic_lacros_google_photos",
				Fixture:           "enrolledLoggedInToCUJUserLacros",
				ExtraSoftwareDeps: []string{"lacros"},
				Timeout:           5 * time.Minute,
				Val:               browser.TypeLacros,
			},
		},
	})
}

// EDUImageEditingCUJ measures the system performance by editing photos on the web.
func EDUImageEditingCUJ(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()

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
	var uiHdl cuj.UIActionHandler
	if tabletMode {
		cleanup, err := display.RotateToLandscape(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to rotate display to landscape: ", err)
		}
		defer cleanup(cleanupCtx)
		if uiHdl, err = cuj.NewTabletActionHandler(ctx, tconn); err != nil {
			s.Fatal("Failed to create tablet action handler: ", err)
		}
	} else {
		if uiHdl, err = cuj.NewClamshellActionHandler(ctx, tconn); err != nil {
			s.Fatal("Failed to create clamshell action handler: ", err)
		}
	}
	defer uiHdl.Close()

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to initialize keyboard input: ", err)
	}
	defer kb.Close()

	testImageLocation := s.DataPath(testImage)
	bt := s.Param().(browser.Type)

	traceConfigPath := ""
	if collect, ok := s.Var("spera.collectTrace"); ok && collect == "enable" {
		traceConfigPath = s.DataPath(cujrecorder.SystemTraceConfigFile)
	}
	googlePhotos := imageeditingcuj.NewGooglePhotos(tconn, uiHdl, kb, cr.Creds().Pass, tabletMode)

	if err := imageeditingcuj.Run(ctx, cr, googlePhotos, bt, tabletMode, s.OutDir(), traceConfigPath, testImage, testImageLocation); err != nil {
		s.Fatal("Failed to run image editing cuj: ", err)
	}
}
