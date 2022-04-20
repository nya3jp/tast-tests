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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/mgs"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FrontlineWorkerCUJ,
		LacrosStatus: testing.LacrosVariantUnknown,
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
					Workload: browsering,
					Tier:     cuj.Basic,
				},
			},
			{
				Name:    "basic_collaborating",
				Timeout: 10 * time.Minute,
				Val: frontlineWorkerParam{
					Workload: collaborating,
					Tier:     cuj.Basic,
				},
			},
		},
	})
}

type frontlineWorkerParam struct {
	Workload workload
	Tier     cuj.Tier
}

// Workload indicates the workload of the case.
type workload uint8

// Browsering is for testing the browsering workload.
// Collaborating is for testing the collaborating workload.
const (
	browsering workload = iota
	collaborating
)

var searchTerms = []string{"Amazon", "CNN", "Disney", "Disney+", "ESPN", "Flickr", "Hulu", "HBO", "HBO GO", "Medium", "Meta", "MLB", "NBA"}

// FrontlineWorkerCUJ measures the system performance.
func FrontlineWorkerCUJ(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*mgs.FixtData).Chrome()
	loginTime := s.FixtValue().(*mgs.FixtData).LoginTime()
	workload := s.Param().(frontlineWorkerParam).Workload

	sampleSheetURL, ok := s.Var("ui.sampleSheetURL")
	if !ok {
		s.Fatal("Require variable ui.sampleSheetURL is not provided")
	}

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
	var uiHdl cuj.UIActionHandler
	if tabletMode {
		if uiHdl, err = cuj.NewTabletActionHandler(ctx, tconn); err != nil {
			s.Fatal("Failed to create tablet action handler: ", err)
		}
		cleanup, err := display.RotateToLandscape(ctx, tconn)
		if err != nil {
			s.Fatal("Failed to rotate display to landscape: ", err)
		}
		defer cleanup(cleanupCtx)
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

	var browserStartTime, appStartTime time.Duration
	testing.ContextLog(ctx, "Start to get browser start time")
	_, browserStartTime, err = cuj.GetBrowserStartTime(ctx, tconn, false, tabletMode, false)
	if err != nil {
		s.Fatal("Failed to get browser start time: ", err)
	}

	// Give 10 seconds to set initial settings. It is critical to ensure
	// cleanupSetting can be executed with a valid context so it has its
	// own cleanup context from other cleanup functions. This is to avoid
	// other cleanup functions executed earlier to use up the context time.
	cleanupSettingsCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cleanupSetting, err := cuj.InitializeSetting(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to set initial settings: ", err)
	}
	defer cleanupSetting(cleanupSettingsCtx)

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

	options := cuj.NewPerformanceCUJOptions()
	recorder, err := cuj.NewRecorder(ctx, cr, nil, options, cuj.MetricConfigs([]*chrome.TestConn{tconn})...)
	if err != nil {
		s.Fatal("Failed to create the recorder: ", err)
	}
	defer recorder.Close(cleanupRecorderCtx)

	numberOfTabs := 13
	if workload == collaborating {
		numberOfTabs = 26
	}

	pv := perf.NewValues()
	if err := recorder.Run(ctx, func(ctx context.Context) error {
		if err := openGoogleTabs(ctx, cr, tconn, uiHdl, numberOfTabs); err != nil {
			return errors.Wrap(err, "failed to open google tabs")
		}
		if workload == browsering {
			if len(searchTerms) > numberOfTabs {
				return errors.New("the number of tabs is less than the search terms")
			}
			if err := enterSearchTerms(ctx, uiHdl, kb, searchTerms); err != nil {
				return errors.Wrap(err, "failed to enter search terms")
			}
		}

		if workload == collaborating {
			for _, idx := range []int{1, 2} {
				if err := scrollTabPage(ctx, uiHdl, idx); err != nil {
					return errors.Wrapf(err, "failed to scroll page within tab index %d", idx)
				}
			}
		}

		sheetName, err = googleSheets.CopySpreadSheet(ctx, sampleSheetURL)
		if err != nil {
			return errors.Wrap(err, "failed to copy spreadsheet")
		}
		if err := uiauto.Combine("operate Google Sheets",
			uiauto.NamedAction("create the pivot table", googleSheets.CreatePivotTable()),
			uiauto.NamedAction("edit the pivot table", googleSheets.EditPivotTable()),
			uiauto.NamedAction("validate the contents of pivot table", googleSheets.ValidatePivotTable()),
		)(ctx); err != nil {
			return err
		}
		googleSheets.Close(ctx)

		if workload == browsering {
			var appGoogleDrive *frontlineworkercuj.GoogleDrive
			if appGoogleDrive, err = frontlineworkercuj.NewGoogleDrive(ctx, tconn, ui, kb); err != nil {
				return errors.Wrap(err, "failed to create Google Drive instance")
			}
			appStartTime, err = appGoogleDrive.Launch(ctx, tconn)
			if err != nil {
				return errors.Wrap(err, "failed to launch Google Drive")
			}
			if err := appGoogleDrive.OpenSpreadSheet(ctx, sheetName); err != nil {
				return errors.Wrap(err, "failed to open the spreadsheet through Google Drive")
			}
		}
		return uiauto.Combine("operate Google Chat app",
			uiauto.NamedAction("launch app", googleChatPWA.Launch),
			uiauto.NamedAction("start a chat", googleChatPWA.StartChat),
		)(ctx)
	}); err != nil {
		s.Fatal("Failed to conduct the recorder task: ", err)
	}

	if err := recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to record the data: ", err)
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
		s.Fatal("Failed to save perf data: ", err)
	}

	if err := recorder.SaveHistograms(outDir); err != nil {
		s.Fatal("Failed to save histogram raw data: ", err)
	}
}

func openGoogleTabs(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, uiHdl cuj.UIActionHandler, numberOfTabs int) error {
	ui := uiauto.New(tconn)
	link := nodewith.Name("English").Role(role.Link)
	for i := 0; i < numberOfTabs; i++ {
		_, err := cr.NewConn(ctx, cuj.GoogleURL)
		if err != nil {
			return errors.Wrapf(err, "the current tab index: %d, failed to open URL: %s", i, cuj.GoogleURL)
		}
		// Since visitors come from different countries, the default language of Google's website is different.
		// Change language to "English" if the English language link exists.
		if err := uiauto.IfSuccessThen(
			ui.WithTimeout(5*time.Second).WaitUntilExists(link),
			uiHdl.Click(link),
		)(ctx); err != nil {
			return err
		}
	}
	return nil
}

func enterSearchTerms(ctx context.Context, uiHdl cuj.UIActionHandler, kb *input.KeyboardEventWriter, searchTerms []string) error {
	for i := 0; i < len(searchTerms); i++ {
		if i == 0 {
			// The first tab will be an empty tab. Therefore, the operation needs to start from the second tab.
			if err := uiHdl.SwitchToNextChromeTab()(ctx); err != nil {
				return err
			}
		}
		testing.ContextLog(ctx, "Switching to chrome tab index: ", i+1)
		if err := uiauto.Combine("enter search term",
			uiHdl.SwitchToNextChromeTab(),
			kb.TypeAction(string(searchTerms[i])),
			kb.AccelAction("Enter"),
		)(ctx); err != nil {
			return err
		}
	}
	return nil
}

func scrollTabPage(ctx context.Context, uiHdl cuj.UIActionHandler, idx int) error {
	scrollActions := uiHdl.ScrollChromePage(ctx)
	if err := uiHdl.SwitchToChromeTabByIndex(idx)(ctx); err != nil {
		return errors.Wrap(err, "failed to switch tab")
	}
	for _, act := range scrollActions {
		if err := act(ctx); err != nil {
			return errors.Wrap(err, "failed to execute action")
		}
	}
	return nil
}
