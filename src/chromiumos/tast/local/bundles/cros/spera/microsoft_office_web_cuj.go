// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package spera

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/spera/productivitycuj"
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
		Func:         MicrosoftOfficeWebCUJ,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures the performance of Microsoft Office web version CUJ",
		Contacts:     []string{"xliu@cienet.com", "alston.huang@cienet.com", "jane.yang@cienet.com"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Vars: []string{
			"spera.ms_username",            // Required. Expecting the username of the "Microsoft" account.
			"spera.ms_password",            // Required. Expecting the password of the "Microsoft" account.
			"spera.sampleMSOfficeSheetURL", // Required. The URL of sample Microsoft Excel. It will be copied to create a new one to perform tests on.
			"spera.cuj_mode",               // Optional. Expecting "tablet" or "clamshell".
			"spera.collectTrace",           // Optional. Expecting "enable" or "disable", default is "disable".
		},
		Data: []string{cujrecorder.SystemTraceConfigFile},
		Params: []testing.Param{
			{
				Name:    "plus",
				Fixture: "loggedInAndKeepState",
				Timeout: 15 * time.Minute,
				Val: productivitycuj.ProductivityParam{
					Tier: cuj.Plus,
				},
			},
			{
				Name:              "plus_lacros",
				Fixture:           "loggedInAndKeepStateLacros",
				Timeout:           15 * time.Minute,
				ExtraSoftwareDeps: []string{"lacros"},
				Val: productivitycuj.ProductivityParam{
					Tier:     cuj.Plus,
					IsLacros: true,
				},
			},
			{
				Name:      "premium",
				Fixture:   "loggedInAndKeepState",
				Timeout:   15 * time.Minute,
				ExtraData: []string{"productivity_cuj_voice_to_text_en.wav"},
				Val: productivitycuj.ProductivityParam{
					Tier: cuj.Premium,
				},
			},
			{
				Name:              "premium_lacros",
				Fixture:           "loggedInAndKeepStateLacros",
				Timeout:           15 * time.Minute,
				ExtraSoftwareDeps: []string{"lacros"},
				ExtraData:         []string{"productivity_cuj_voice_to_text_en.wav"},
				Val: productivitycuj.ProductivityParam{
					Tier:     cuj.Premium,
					IsLacros: true,
				},
			},
		},
	})
}

func MicrosoftOfficeWebCUJ(ctx context.Context, s *testing.State) {
	p := s.Param().(productivitycuj.ProductivityParam)
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	sampleSheetURL, ok := s.Var("spera.sampleMSOfficeSheetURL")
	if !ok {
		s.Fatal("Require variable spera.sampleMSOfficeSheetURL is not provided")
	}

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

	username := s.RequiredVar("spera.ms_username")
	password := s.RequiredVar("spera.ms_password")

	office := productivitycuj.NewMicrosoftWebOffice(tconn, uiHdl, kb, tabletMode, p.IsLacros, username, password)

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
	if collect, ok := s.Var("spera.collectTrace"); ok && collect == "enable" {
		traceConfigPath = s.DataPath(cujrecorder.SystemTraceConfigFile)
	}
	if err := productivitycuj.Run(ctx, cr, office, p.Tier, tabletMode, bt, s.OutDir(), traceConfigPath, sampleSheetURL, expectedText, testFileLocation); err != nil {
		s.Fatal("Failed to run productivity cuj: ", err)
	}
}
