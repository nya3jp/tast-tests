// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package spera

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/spera/frontlineworkercuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/mgs"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FrontlineWorkerCUJ,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Measures the performance of Frontline Worker CUJ",
		Contacts:     []string{"xliu@cienet.com", "alston.huang@cienet.com", "cienet-development@googlegroups.com"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Vars: []string{
			"spera.username",       // Required. It is necessary to have account to use Google Sheets.
			"spera.password",       // Required. It is necessary to have account to use Google Sheets.
			"spera.sampleSheetURL", // Required. The URL of sample Google Sheet. It will be copied to create a new one to perform tests on.
			"spera.cuj_mode",       // Optional. Expecting "tablet" or "clamshell".
			"spera.collectTrace",   // Optional. Expecting "enable" or "disable", default is "disable".
		},
		Data: []string{cujrecorder.SystemTraceConfigFile},
		Params: []testing.Param{
			{
				Name:    "basic_browsing",
				Timeout: 10 * time.Minute,
				Fixture: fixture.ManagedGuestSessionWithPWA,
				Val: frontlineWorkerParam{
					workload: browsering,
					tier:     cuj.Basic,
				},
			},
			{
				Name:    "basic_lacros_browsing",
				Timeout: 10 * time.Minute,
				Fixture: fixture.ManagedGuestSessionWithPWALacros,
				Val: frontlineWorkerParam{
					workload: browsering,
					tier:     cuj.Basic,
				},
			},
			{
				Name:    "basic_collaborating",
				Timeout: 10 * time.Minute,
				Fixture: fixture.ManagedGuestSessionWithPWA,
				Val: frontlineWorkerParam{
					workload: collaborating,
					tier:     cuj.Basic,
				},
			},
			{
				Name:    "basic_lacros_collaborating",
				Timeout: 10 * time.Minute,
				Fixture: fixture.ManagedGuestSessionWithPWALacros,
				Val: frontlineWorkerParam{
					workload: collaborating,
					tier:     cuj.Basic,
				},
			},
		},
	})
}

type frontlineWorkerParam struct {
	workload workload
	tier     cuj.Tier
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
	bt := s.FixtValue().(*mgs.FixtData).BrowserType()
	workload := s.Param().(frontlineWorkerParam).workload

	sampleSheetURL, ok := s.Var("spera.sampleSheetURL")
	if !ok {
		s.Fatal("Require variable spera.sampleSheetURL is not provided")
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
	l, browserStartTime, err := cuj.GetBrowserStartTime(ctx, tconn, false, tabletMode, bt)
	if err != nil {
		s.Fatal("Failed to get browser start time: ", err)
	}
	br := cr.Browser()
	if l != nil {
		defer l.Close(ctx)
		br = l.Browser()
	}
	bTconn, err := br.TestAPIConn(ctx)
	if err != nil {
		s.Fatalf("Failed to create Test API connection for %v browser: %v", bt, err)
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

	defer browser.CloseAllTabs(ctx, tconn)

	outDir := s.OutDir()
	account, password := s.RequiredVar("spera.username"), s.RequiredVar("spera.password")

	googleChatPWA := frontlineworkercuj.NewGoogleChat(ctx, br, ui, uiHdl, kb)
	googleSheets := frontlineworkercuj.NewGoogleSheets(ctx, tconn, br, ui, uiHdl, kb, account, password)

	var sheetName string
	// Shorten the context to clean up the files created in the test case.
	cleanUpResourceCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()
	defer googleSheets.RemoveFile(cleanUpResourceCtx, &sheetName)

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, outDir, s.HasError, cr, "ui_dump")

	// Shorten the context to cleanup recorder.
	cleanupRecorderCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	options := cujrecorder.NewPerformanceCUJOptions()
	recorder, err := cujrecorder.NewRecorder(ctx, cr, bTconn, nil, options)
	if err != nil {
		s.Fatal("Failed to create the recorder: ", err)
	}
	defer recorder.Close(cleanupRecorderCtx)
	if err := cuj.AddPerformanceCUJMetrics(bt, tconn, bTconn, recorder); err != nil {
		s.Fatal("Failed to add metrics to recorder: ", err)
	}
	if collect, ok := s.Var("spera.collectTrace"); ok && collect == "enable" {
		recorder.EnableTracing(s.OutDir(), s.DataPath(cujrecorder.SystemTraceConfigFile))
	}
	numberOfTabs := 13
	if workload == collaborating {
		numberOfTabs = 26
	}

	pv := perf.NewValues()
	if err := recorder.Run(ctx, func(ctx context.Context) (retErr error) {
		if err := openGoogleTabs(ctx, br, uiHdl, numberOfTabs); err != nil {
			return errors.Wrap(err, "failed to open google tabs")
		}
		if err := cuj.MaximizeBrowserWindow(ctx, tconn, tabletMode, "Google"); err != nil {
			return errors.Wrap(err, "failed to maximize the window")
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
			for i := 0; i < 2; i++ {
				if err := scrollTabPage(ctx, uiHdl); err != nil {
					return errors.Wrapf(err, "failed to scroll page within tab index %d", i)
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
			appGoogleDrive := frontlineworkercuj.NewGoogleDrive(ctx, tconn, ui)
			appStartTime, err = appGoogleDrive.Launch(ctx, tconn)
			if err != nil {
				return errors.Wrap(err, "failed to launch Google Drive")
			}
			defer appGoogleDrive.Close(ctx)
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, outDir, func() bool { return retErr != nil }, cr, "ui_tree")

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

func openGoogleTabs(ctx context.Context, br *browser.Browser, uiHdl cuj.UIActionHandler, numberOfTabs int) (err error) {
	googleURL := cuj.GoogleURL + "/?hl=en"
	for i := 0; i < numberOfTabs; i++ {
		testing.ContextLog(ctx, "Opening tab ", i)
		var conn *chrome.Conn
		if i == 0 {
			conn, err = uiHdl.NewChromeTab(ctx, br, googleURL, false)
		} else {
			// uiHdl.NewChromeTab will fail to open new tab in lacros on tablet due to the clicking wrong location issue.
			conn, err = br.NewConn(ctx, googleURL)
		}
		if err != nil {
			return errors.Wrapf(err, "the current tab index: %d, failed to open URL: %s", i, cuj.GoogleURL)
		}
		if err := webutil.WaitForRender(ctx, conn, 2*time.Minute); err != nil {
			return errors.Wrap(err, "failed to wait for render to finish")
		}
	}
	return nil
}

func enterSearchTerms(ctx context.Context, uiHdl cuj.UIActionHandler, kb *input.KeyboardEventWriter, searchTerms []string) error {
	for i := 0; i < len(searchTerms); i++ {
		testing.ContextLog(ctx, "Switching to chrome tab index: ", i)
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

func scrollTabPage(ctx context.Context, uiHdl cuj.UIActionHandler) error {
	scrollActions := uiHdl.ScrollChromePage(ctx)
	if err := uiHdl.SwitchToNextChromeTab()(ctx); err != nil {
		return err
	}
	for _, act := range scrollActions {
		if err := act(ctx); err != nil {
			return errors.Wrap(err, "failed to execute action")
		}
	}
	return nil
}
