// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/conference"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         GoogleSlides,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Reproduce chrome.TestConn hang issues for Google Slides",
		Contacts:     []string{"xliu@cienet.com"},
		SoftwareDeps: []string{"chrome", "arc"},
		Timeout:      3 * time.Minute,
		Params: []testing.Param{
			{
				Name:    "ash",
				Fixture: "loggedInAndKeepState",
				Val:     browser.TypeAsh,
			}, {
				Name:              "lacros",
				Fixture:           "loggedInAndKeepStateLacrosWithARC",
				ExtraSoftwareDeps: []string{"lacros"},
				Val:               browser.TypeLacros,
			},
		},
	})
}

func GoogleSlides(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	p := s.Param().(browser.Type)

	uiHandler, err := cuj.NewClamshellActionHandler(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to create clamshell action handler: ", err)
	}
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to initialize keyboard input: ", err)
	}
	defer kb.Close()

	isLacros := p == browser.TypeLacros
	// Creates a Google Meet conference instance which implements conference.Conference methods
	// which provides conference operations.
	gmcli := conference.NewGoogleMeetConference(cr, tconn, kb, uiHandler, false, false, isLacros, 0, "", "", s.OutDir())
	defer gmcli.End(ctx)
	// Shorten context a bit to allow for cleanup if Run fails.
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()

	if err := testRun(ctx, cr, gmcli, "plus", s.OutDir(), false, isLacros, 0); err != nil {
		s.Fatal("Failed to run conference: ", err)
	}
}

// testRun runs the specified user scenario in conference room with different CUJ tiers.
func testRun(ctx context.Context, cr *chrome.Chrome, conf conference.Conference, tier, outDir string, tabletMode, isLacros bool, roomSize int) (retErr error) {
	// Shorten context a bit to allow for cleanup.
	cleanUpCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Dump the UI tree to the service/faillog subdirectory.
	// Don't dump directly into outDir
	// because it might be overridden by the test faillog after pulled back to remote server.
	defer faillog.DumpUITreeWithScreenshotOnError(cleanUpCtx, filepath.Join(outDir, "service"), func() bool { return retErr != nil }, cr, "ui_dump")

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to the test API connection")
	}

	testing.ContextLog(ctx, "Start to get browser start time")
	l, _, err := cuj.GetBrowserStartTime(ctx, tconn, true, tabletMode, isLacros)
	if err != nil {
		return errors.Wrap(err, "failed to get browser start time")
	}
	br := cr.Browser()
	tconns := []*chrome.TestConn{tconn}
	if isLacros {
		br = l.Browser()
		bTconn, err := l.TestAPIConn(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get lacros test API conn")
		}
		tconns = append(tconns, bTconn)
	}
	conf.SetBrowser(br)

	// Plus and premium tier.
	if err := conf.Presenting(ctx, conference.GoogleSlides); err != nil {
		return err
	}
	return nil
}
