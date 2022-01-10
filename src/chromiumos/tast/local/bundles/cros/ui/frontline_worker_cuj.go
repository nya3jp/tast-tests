// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/frontlineworkercuj"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/mgs"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FrontlineWorkerCUJ,
		Desc:         "Measures the performance of Frontline Worker CUJ",
		Contacts:     []string{"xliu@cienet.com", "alston.huang@cienet.com"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Fixture:      fixture.ManagedGuestSessionWithExtensions,
		Vars: []string{
			"ui.cujAccountPool", // Required. It is necessary to have account to use Google Sheets.
			"ui.sampleSheetURL", // Required. The URL of sample Google Sheet. It will be copied to create a new one to perform tests on.
			"ui.cuj_mode",       // Optional. Expecting "tablet" or "clamshell".
		},
		Params: []testing.Param{
			{
				Name:    "basic_browsing",
				Timeout: 10 * time.Minute,
				Val: frontlineWorkerParam{
					Workload: frontlineworkercuj.Browsering,
					Tier:     cuj.Basic,
				},
			},
			{
				Name:    "basic_collaborating",
				Timeout: 10 * time.Minute,
				Val: frontlineWorkerParam{
					Workload: frontlineworkercuj.Collaborating,
					Tier:     cuj.Basic,
				},
			},
		},
	})
}

type frontlineWorkerParam struct {
	Workload frontlineworkercuj.Workload
	Tier     cuj.Tier
}

var searchTerms = []string{"Amazon", "CNN", "Disney", "Disney+", "ESPN", "Flickr", "Hulu", "HBO", "HBO GO", "Medium", "Meta", "MLB", "NBA"}

// FrontlineWorkerCUJ measures the system performance.
func FrontlineWorkerCUJ(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*mgs.FixtData).Chrome()
	loginTime := s.FixtValue().(*mgs.FixtData).LoginTime()
	workload := s.Param().(frontlineWorkerParam).Workload

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}
	ui := uiauto.New(tconn)

	// Shorten context a bit to allow for cleanup if Run fails.
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
	if tabletMode {
		cleanup, err := display.RotateToLandscape(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to rotate display to landscape: ", err)
		}
		defer cleanup(cleanupCtx)
	}

	sampleSheetURL, ok := s.Var("ui.sampleSheetURL")
	if !ok {
		s.Fatal("Require variable ui.sampleSheetURL is not provided")
	}

	var browserStartTime, appStartTime time.Duration
	testing.ContextLog(ctx, "Start to get browser start time")
	browserStartTime, err = cuj.GetBrowserStartTime(ctx, cr, tconn, false, tabletMode)
	if err != nil {
		s.Fatal("Failed to get browser start time")
	}

	// Shorten the context to resume battery charging.
	cleanUpBatteryCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()
	// Put battery under discharge in order to collect the power consumption of the test.
	setBatteryNormal, err := cuj.SetBatteryDischarge(ctx, 50)
	if err != nil {
		s.Fatal("Failed to set battery discharge")
	}
	defer setBatteryNormal(cleanUpBatteryCtx)

	// Give 10 seconds to set initial settings. It is critical to ensure
	// cleanupSetting can be executed with a valid context so it has its
	// own cleanup context from other cleanup functions. This is to avoid
	// other cleanup functions executed earlier to use up the context time.
	cleanupSettingsCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cleanupSetting, err := cuj.InitializeSetting(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to set initial settings")
	}
	defer cleanupSetting(cleanupSettingsCtx)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to initialize keyboard input")
	}
	defer kb.Close()

	var uiHdl cuj.UIActionHandler
	if tabletMode {
		if uiHdl, err = cuj.NewTabletActionHandler(ctx, tconn); err != nil {
			s.Fatal("Failed to create tablet action handler")
		}
	} else {
		if uiHdl, err = cuj.NewClamshellActionHandler(ctx, tconn); err != nil {
			s.Fatal("Failed to create clamshell action handler")
		}
	}
	defer uiHdl.Close()
	defer cuj.CloseBrowserTabs(ctx, tconn)

	outDir := s.OutDir()
	credentials := strings.Split(s.RequiredVar("ui.cujAccountPool"), ":")
	account, password := credentials[0], credentials[1]

	googleChatPWA := frontlineworkercuj.NewGoogleChat(ctx, cr, tconn, ui, uiHdl, kb)
	googleSheets := frontlineworkercuj.NewGoogleSheets(ctx, cr, tconn, ui, uiHdl, kb, account, password)

	var sheetName string
	// Shorten the context to clean up the files created in the test case.
	cleanUpResourceCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()
	defer googleSheets.RemoveFile(cleanUpResourceCtx, &sheetName)

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, outDir, func() bool { return err != nil }, cr, "ui_dump")

	// Shorten the context to cleanup recorder.
	cleanupRecorderCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	recorder, err := cuj.NewRecorder(ctx, cr, nil, cuj.MetricConfigs()...)
	if err != nil {
		s.Fatal("Failed to create the recorder")
	}
	defer recorder.Close(cleanupRecorderCtx)

	numberOfTabs := 13
	if workload == frontlineworkercuj.Collaborating {
		numberOfTabs = 26
	}

	pv := perf.NewValues()
	if err := recorder.Run(ctx, func(ctx context.Context) error {
		if err := frontlineworkercuj.OpenGoogleTabs(ctx, cr, tconn, uiHdl, numberOfTabs); err != nil {
			return err
		}
		if workload == frontlineworkercuj.Browsering {
			if len(searchTerms) > numberOfTabs {
				return errors.New("the number of tabs is less than the search terms")
			}
			if err := frontlineworkercuj.EnterSearchTerms(ctx, uiHdl, kb, searchTerms); err != nil {
				return err
			}
		}

		if workload == frontlineworkercuj.Collaborating {
			for _, idx := range []int{1, 2} {
				if err := frontlineworkercuj.ScrollTabPage(ctx, uiHdl, idx); err != nil {
					return errors.Wrapf(err, "failed to scroll page within tab index %d", idx)
				}
			}
		}

		sheetName, err = googleSheets.CopySpreadSheet(ctx, sampleSheetURL)
		if err != nil {
			return errors.Wrap(err, "failed to copy spreadsheet")
		}
		if err := googleSheets.CreatePivotTable(ctx); err != nil {
			return errors.Wrap(err, "failed to pivot the table")
		}
		if err := googleSheets.EditPivotTable(ctx); err != nil {
			return errors.Wrap(err, "failed to edit the pivot table")
		}
		if err := googleSheets.ValidatePivotTable(ctx); err != nil {
			return errors.Wrap(err, "failed to validate the contents of pivot table")
		}
		googleSheets.Close(ctx)

		if workload == frontlineworkercuj.Browsering {
			var appGoogleDrive *frontlineworkercuj.GoogleDrive
			if appGoogleDrive, err = frontlineworkercuj.NewGoogleDrive(ctx, tconn, ui, kb); err != nil {
				return errors.Wrap(err, "failed to create Google Drive instance")
			}
			appStartTime, err = appGoogleDrive.Launch(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to launch Google Drive")
			}
			if err := appGoogleDrive.OpenSpreadSheet(ctx, sheetName); err != nil {
				return errors.Wrap(err, "failed to open the spreadsheet through Google Drive")
			}
		}

		if err := googleChatPWA.Launch(ctx); err != nil {
			return errors.Wrap(err, "failed to launch Google Chat app")
		}
		if err := googleChatPWA.StartChat(ctx); err != nil {
			return errors.Wrap(err, "failed to start a chat")
		}
		return nil
	}); err != nil {
		s.Fatal("Failed to conduct the recorder task")
	}

	if err := recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to record the data")
	}

	pv.Set(perf.Metric{
		Name:      "Browser.StartTime",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, float64(browserStartTime.Milliseconds()))

	pv.Set(perf.Metric{
		Name:      "User.LoginTime",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, float64(loginTime.Milliseconds()))

	pv.Set(perf.Metric{
		Name:      "Apps.StartTime",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, float64(appStartTime.Milliseconds()))

	if err := pv.Save(outDir); err != nil {
		s.Fatal("Failed to save perf data")
	}

	if err := recorder.SaveHistograms(outDir); err != nil {
		s.Fatal("Failed to save histogram raw data")
	}
}
