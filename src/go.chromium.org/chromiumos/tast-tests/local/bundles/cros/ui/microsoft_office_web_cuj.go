// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"go.chromium.org/chromiumos/tast/ctxutil"
	"go.chromium.org/chromiumos/tast-tests/local/bundles/cros/ui/cuj"
	"go.chromium.org/chromiumos/tast-tests/local/bundles/cros/ui/productivitycuj"
	"go.chromium.org/chromiumos/tast-tests/local/chrome"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/ash"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/browser"
	"go.chromium.org/chromiumos/tast-tests/local/chrome/display"
	"go.chromium.org/chromiumos/tast-tests/local/input"
	"go.chromium.org/chromiumos/tast/testing"
	"go.chromium.org/chromiumos/tast/testing/hwdep"
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
			"ui.ms_username",            // Required. Expecting the username of the "Microsoft" account.
			"ui.ms_password",            // Required. Expecting the password of the "Microsoft" account.
			"ui.sampleMSOfficeSheetURL", // Required. The URL of sample Microsoft Excel. It will be copied to create a new one to perform tests on.
			"ui.cuj_mode",               // Optional. Expecting "tablet" or "clamshell".
		},
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
	sampleSheetURL, ok := s.Var("ui.sampleMSOfficeSheetURL")
	if !ok {
		s.Fatal("Require variable ui.sampleMSOfficeSheetURL is not provided")
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

	username := s.RequiredVar("ui.ms_username")
	password := s.RequiredVar("ui.ms_password")

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
	if err := productivitycuj.Run(ctx, cr, office, p.Tier, tabletMode, bt, s.OutDir(), sampleSheetURL, expectedText, testFileLocation); err != nil {
		s.Fatal("Failed to run productivity cuj: ", err)
	}
}
