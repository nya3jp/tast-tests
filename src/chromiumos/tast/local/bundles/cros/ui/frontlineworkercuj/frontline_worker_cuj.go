// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package frontlineworkercuj

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil/mgs"
	"chromiumos/tast/testing"
)

const (
	// defaultUIWaitTime indicates the default time to wait for UI elements to appear.
	defaultUIWaitTime = 5 * time.Second
	// longerUIWaitTime indicates the time to wait for some UI elements that need more time to appear.
	longerUIWaitTime = time.Minute
)

// Workload indicates the workload of the case.
type Workload uint8

// Browsering is for testing the browsering workload.
// Collaborating is for testing the collaborating workload.
const (
	Browsering Workload = iota
	Collaborating
)

var searchTerms = []string{"Amazon", "CNN", "Disney", "Disney+", "ESPN", "Flickr", "Hulu", "HBO", "HBO GO", "Medium", "Meta", "MLB", "NBA"}

// Run runs the FrontlineWorkerCUJ test.
func Run(ctx context.Context, s *testing.State, cr *chrome.Chrome, workload Workload, isTablet bool) error {
	loginTime := s.FixtValue().(*mgs.FixtData).LoginTime()
	outDir := s.OutDir()
	sampleSheetURL, ok := s.Var("ui.sampleSheetURL")
	if !ok {
		return errors.New("Require variable ui.sampleSheetURL is not provided")
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to connect to the test API connection")
	}
	ui := uiauto.New(tconn)

	var browserStartTime, appStartTime time.Duration
	testing.ContextLog(ctx, "Start to get browser start time")
	browserStartTime, err = cuj.GetBrowserStartTime(ctx, cr, tconn, isTablet)
	if err != nil {
		return errors.Wrap(err, "failed to get browser start time")
	}

	// Shorten the context to resume battery charging.
	cleanUpBatteryCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()
	// Put battery under discharge in order to collect the power consumption of the test.
	setBatteryNormal, err := cuj.SetBatteryDischarge(ctx, 50)
	if err != nil {
		return errors.Wrap(err, "failed to set battery discharge")
	}
	defer setBatteryNormal(cleanUpBatteryCtx)

	// Give 10 seconds to set initial settings. It is critical to ensure
	// cleanupSetting can be executed with a valid context so it has its
	// own cleanup context from other cleanup functions. This is to avoid
	// other cleanup functions executed earlier to use up the context time.
	cleanupSettingsCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cleanupSetting, err := cuj.InitializeSetting(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to set initial settings")
	}
	defer cleanupSetting(cleanupSettingsCtx)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to initialize keyboard input")
	}
	defer kb.Close()

	var uiHdl cuj.UIActionHandler
	if isTablet {
		if uiHdl, err = cuj.NewTabletActionHandler(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to create tablet action handler")
		}
	} else {
		if uiHdl, err = cuj.NewClamshellActionHandler(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to create clamshell action handler")
		}
	}
	defer uiHdl.Close()

	defer cuj.CloseBrowserTabs(ctx, tconn)

	credentials := strings.Split(s.RequiredVar("ui.cujAccountPool"), ":")
	account, password := credentials[0], credentials[1]

	googleChatPWA := NewGoogleChat(ctx, cr, tconn, ui, uiHdl, kb)
	googleSheets := NewGoogleSheets(ctx, cr, tconn, ui, uiHdl, kb, account, password)

	var sheetName string
	// Shorten the context to clean up the files created in the test case.
	cleanUpResourceCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()
	defer googleSheets.removeFile(cleanUpResourceCtx, &sheetName)

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, outDir, func() bool { return err != nil }, cr, "ui_dump")

	// Shorten the context to cleanup recorder.
	cleanupRecorderCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	recorder, err := cuj.NewRecorder(ctx, cr, nil, cuj.MetricConfigs()...)
	if err != nil {
		return errors.Wrap(err, "failed to create the recorder")
	}
	defer recorder.Close(cleanupRecorderCtx)

	numberOfTabs := 13
	if workload == Collaborating {
		numberOfTabs = 26
	}

	pv := perf.NewValues()
	if err := recorder.Run(ctx, func(ctx context.Context) error {
		if err := openTabs(ctx, cr, numberOfTabs); err != nil {
			return err
		}
		if workload == Browsering {
			if len(searchTerms) > numberOfTabs {
				return errors.New("the number of tabs is less than the search terms")
			}
			if err := enterSearchTerms(ctx, uiHdl, kb, searchTerms); err != nil {
				return err
			}
		}

		if workload == Collaborating {
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

		if workload == Browsering {
			var appGoogleDrive *GoogleDrive
			if appGoogleDrive, err = NewGoogleDrive(ctx, tconn, ui, kb); err != nil {
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
		return errors.Wrap(err, "failed to conduct the recorder task")
	}

	if err := recorder.Record(ctx, pv); err != nil {
		return errors.Wrap(err, "failed to record the data")
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
		return errors.Wrap(err, "failed to save perf data")
	}

	if err := recorder.SaveHistograms(outDir); err != nil {
		return errors.Wrap(err, "failed to save histogram raw data")
	}

	return nil
}

// openTabs opens the specified number of chrome tabs.
func openTabs(ctx context.Context, cr *chrome.Chrome, numberOfTabs int) error {
	for i := 0; i < numberOfTabs; i++ {
		_, err := cr.NewConn(ctx, cuj.GoogleURL)
		if err != nil {
			return errors.Wrapf(err, "the current tab index: %d, failed to open URL: %s", i, cuj.GoogleURL)
		}
	}
	return nil
}

// enterSearchTerms enters search terms on each tab.
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

// scrollTabPage scrolls the specified tab index of the webpage.
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
