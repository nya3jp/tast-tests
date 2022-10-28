// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/ui/productivitycuj"
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

func init() {
	testing.AddTest(&testing.Test{
		// TODO (b/242590511): Deprecated after moving all performance cuj test cases to chromiumos/tast/local/bundles/cros/spera directory.
		Func:         GoogleDocsWebCUJ,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures the performance of Google Docs web version CUJ",
		Contacts:     []string{"xliu@cienet.com", "alston.huang@cienet.com", "jane.yang@cienet.com"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Vars: []string{
			"ui.sampleGDocSheetURL", // Required. The URL of sample Google Sheet. It will be copied to create a new one to perform tests on.
			"ui.cuj_mode",           // Optional. Expecting "tablet" or "clamshell".
			"ui.collectTrace",       // Optional. Expecting "enable" or "disable", default is "disable".
		},
		Data: []string{cujrecorder.SystemTraceConfigFile},
		Params: []testing.Param{
			{
				Name:    "basic",
				Fixture: "loggedInAndKeepState",
				Timeout: 15 * time.Minute,
				Val: productivitycuj.ProductivityParam{
					Tier: cuj.Basic,
				},
			},
			{
				Name:              "basic_lacros",
				Fixture:           "loggedInAndKeepStateLacros",
				Timeout:           15 * time.Minute,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: productivitycuj.ProductivityParam{
					Tier:     cuj.Basic,
					IsLacros: true,
				},
			},
			{
				Name:      "premium",
				Fixture:   "loggedInAndKeepState",
				ExtraData: []string{"productivity_cuj_voice_to_text_en.wav"},
				Timeout:   15 * time.Minute,
				Val: productivitycuj.ProductivityParam{
					Tier: cuj.Premium,
				},
			},
			{
				Name:              "premium_lacros",
				Fixture:           "loggedInAndKeepStateLacros",
				ExtraData:         []string{"productivity_cuj_voice_to_text_en.wav"},
				Timeout:           15 * time.Minute,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: productivitycuj.ProductivityParam{
					Tier:     cuj.Premium,
					IsLacros: true,
				},
			},
		},
	})
}

func GoogleDocsWebCUJ(ctx context.Context, s *testing.State) {
	p := s.Param().(productivitycuj.ProductivityParam)
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	sampleSheetURL, ok := s.Var("ui.sampleGDocSheetURL")
	if !ok {
		s.Fatal("Require variable ui.sampleGDocSheetURL is not provided")
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()

	var tabletMode bool
	if mode, ok := s.Var("ui.cuj_mode"); ok {
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

	office := productivitycuj.NewGoogleDocs(tconn, kb, uiHdl, tabletMode)

	var expectedText, testFileLocation string
	if p.Tier == cuj.Premium {
		expectedText = "Mary had a little lamb whose fleece was white as snow And everywhere that Mary went the lamb was sure to go"
		testFileLocation = s.DataPath("productivity_cuj_voice_to_text_en.wav")
	}
	bt := browser.TypeAsh
	if p.IsLacros {
		bt = browser.TypeLacros
	}
	traceConfigPath := ""
	if collect, ok := s.Var("ui.collectTrace"); ok && collect == "enable" {
		traceConfigPath = s.DataPath(cujrecorder.SystemTraceConfigFile)
	}
	if err := productivitycuj.Run(ctx, cr, office, p.Tier, tabletMode, bt, s.OutDir(), traceConfigPath, sampleSheetURL, expectedText, testFileLocation); err != nil {
		s.Fatal("Failed to run productivity cuj: ", err)
	}
}
