// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GoogleSheetsCUJ,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures the performance of critical user journey for Google Sheets",
		Contacts:     []string{"yichenz@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "arc"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      10 * time.Minute,
		Params: []testing.Param{{
			Val:     browser.TypeAsh,
			Fixture: "loggedInToCUJUser",
		}, {
			Name:              "lacros",
			Val:               browser.TypeLacros,
			Fixture:           "loggedInToCUJUserLacros",
			ExtraSoftwareDeps: []string{"lacros"},
		}},
	})
}

func GoogleSheetsCUJ(ctx context.Context, s *testing.State) {
	const (
		timeout       = 10 * time.Second
		sheetURL      = "https://docs.google.com/spreadsheets/d/1I9jmmdWkBaH6Bdltc2j5KVSyrJYNAhwBqMmvTdmVOgM/edit?usp=sharing&resourcekey=0-60wBsoTfOkoQ6t4yx2w7FQ"
		scrollTimeout = 7 * time.Minute
	)

	// Shorten context a bit to allow for cleanup.
	closeCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Second)
	defer cancel()

	bt := s.Param().(browser.Type)

	var cs ash.ConnSource
	var cr *chrome.Chrome

	if bt == browser.TypeAsh {
		cr = s.FixtValue().(cuj.FixtureData).Chrome
		cs = cr
	} else {
		var err error
		var l *lacros.Lacros
		cr, l, cs, err = lacros.Setup(ctx, s.FixtValue(), bt)
		if err != nil {
			s.Fatal("Failed to initialize test: ", err)
		}
		defer lacros.CloseLacros(ctx, l)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}
	defer faillog.DumpUITreeOnError(closeCtx, s.OutDir(), s.HasError, tconn)

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to set tablet mode to false: ", err)
	}
	defer cleanup(closeCtx)

	ui := uiauto.New(tconn)

	// Open Google Sheets file.
	sheetConn, err := cs.NewConn(ctx, sheetURL, browser.WithNewWindow())
	if err != nil {
		s.Fatal("Failed to open the Google Sheets website: ", err)
	}
	defer sheetConn.Close()
	s.Log("Creating a Google Sheets window")
	// Pop-up content regarding view history privacy might show up.
	privacyButton := nodewith.Name("I understand").Role(role.Button)
	if err := ui.WaitUntilExists(privacyButton)(ctx); err == nil {
		if err := ui.LeftClick(privacyButton)(ctx); err != nil {
			s.Fatal("Failed to click the spreadsheet privacy button: ", err)
		}
	}

	recorder, err := cuj.NewRecorder(ctx, cr, nil)
	if err != nil {
		s.Fatal("Failed to create a CUJ recorder: ", err)
	}
	defer recorder.Close(closeCtx)

	// Create a virtual trackpad.
	tpw, err := input.Trackpad(ctx)
	if err != nil {
		s.Fatal("Failed to create a trackpad device: ", err)
	}
	defer tpw.Close()
	tw, err := tpw.NewMultiTouchWriter(2)
	if err != nil {
		s.Fatal("Failed to create a multi touch writer: ", err)
	}
	defer tw.Close()

	if err := recorder.Run(ctx, func(ctx context.Context) error {
		fingerSpacing := tpw.Width() / 4
		end := time.Now().Add(scrollTimeout)
		// Swipe and scroll down the spreadsheet.
		s.Logf("Scrolling down the Google Sheets file for %d minutes", scrollTimeout)
		for end.Sub(time.Now()).Seconds() > 0 {
			if err := tw.Swipe(ctx, tpw.Width()/2, 1,
				tpw.Width()/2, tpw.Height()-1, fingerSpacing,
				2, 500*time.Millisecond); err != nil {
				return errors.Wrap(err, "failed to swipe")
			}
		}
		return nil
	}); err != nil {
		s.Fatal("Failed to run the test scenario: ", err)
	}

	pv := perf.NewValues()
	if err := recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to record the data: ", err)
	}
	if pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to save the perf data: ", err)
	}
}
