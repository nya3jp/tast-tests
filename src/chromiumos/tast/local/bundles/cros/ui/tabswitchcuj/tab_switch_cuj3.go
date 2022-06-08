// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package tabswitchcuj contains the test code for TabSwitchCUJ. The test is
// extracted into this package to be shared between TabSwitchCUJRecorder and
// TabSwitchCUJ.
package tabswitchcuj

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
)

// TabSwitchVariables hold all the necessary variables used by the test
type TabSwitchVariables struct {
	param    TabSwitchParam // Test Parameters
	WebPages []WebPage      // List of sites to visit

	cr              *chrome.Chrome        // Ash Fixture
	l               *lacros.Lacros        // Lacros Fixture
	cs              ash.ConnSource        // Direct Connection To Browser
	browserTestConn *chrome.TestConn      // Test Connection To Browser (for metrics)
	recorder        *cujrecorder.Recorder // Recorder to collect metrics
}

// WebPage is the struct which holds the info used to visit new sites in the test.
type WebPage struct {
	name       string // Display Name of the Website
	startURL   string // Base URL to the Website
	urlPattern string // RegExp Pattern to Open Relevant Links on the Website
}

// CoreTestDurationInMins is the variable ensures that the core part of the test
// runs for AT LEAST this number, in minutes.
//
// The actual test duration could be longer depending on the various setup that is
// required for the test body.
const CoreTestDurationInMins = 10

// The struct that holds all the variables the test needs to access
var test TabSwitchVariables

// Making testing state global to modularize code
var state *testing.State

func runSetup(ctx context.Context, s *testing.State, param TabSwitchParam) (TabSwitchVariables, error) {
	var cr *chrome.Chrome
	var cs ash.ConnSource
	var l *lacros.Lacros
	var browserTestConn *chrome.TestConn
	var err error

	state = s
	WebPages := getTestWebpages()

	if param.BrowserType == browser.TypeAsh {
		cr = s.PreValue().(*chrome.Chrome)
		cs = cr

		if browserTestConn, err = cr.TestAPIConn(ctx); err != nil {
			s.Fatal("Failed to get TestAPIConn: ", err)
		}
	} else {
		cr, l, cs, err = lacros.Setup(ctx, s.FixtValue(), param.BrowserType)
		if err != nil {
			s.Fatal("Failed to initialize test: ", err)
		}

		if browserTestConn, err = l.TestAPIConn(ctx); err != nil {
			s.Fatal("Failed to get lacros TestAPIConn: ", err)
		}
	}

	recorder, err := cujrecorder.NewRecorder(ctx, cr, nil, cujrecorder.RecorderOptions{})
	if err != nil {
		s.Fatal("Failed to create a recorder: ", err)
	}

	if err := recorder.AddCollectedMetrics(browserTestConn, cujrecorder.DeprecatedMetricConfigs()...); err != nil {
		s.Fatal("Failed to add metrics to recorder: ", err)
	}

	if param.Tracing {
		recorder.EnableTracing(s.OutDir(), s.DataPath(cujrecorder.SystemTraceConfigFile))
	}

	return TabSwitchVariables{param, WebPages, cr, l, cs, browserTestConn, recorder}, nil
}

func getTestWebpages() []WebPage {
	CNN := WebPage{
		name:       "CNN",
		startURL:   "https://cnn.com",
		urlPattern: `^.*://www.cnn.com/\d{4}/\d{2}/\d{2}/`,
	}

	Reddit := WebPage{
		"Reddit",
		"https://reddit.com",
		`^.*://www.reddit.com/r/[^/]+/comments/[^/]+/`,
	}

	return []WebPage{CNN, Reddit}
}

func muteDevice(ctx context.Context, s *testing.State) {
	kw, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open the keyboard: ", err)
	}
	defer kw.Close()

	// The custom variable for the developer to mute the device before the test,
	// so it doesn't make any noise when some of the visited pages play video.
	if _, ok := s.Var("mute"); ok {
		topRow, err := input.KeyboardTopRowLayout(ctx, kw)
		if err != nil {
			s.Fatal("Failed to obtain the top-row layout: ", err)
		}
		if err = kw.Accel(ctx, topRow.VolumeMute); err != nil {
			s.Fatal("Failed to mute: ", err)
		}
	}
}

func getEndTimeForTest(totalDurationInMins, numSubTests int) time.Time {
	if numSubTests < 1 {
		return time.Time{} // Return invalid end time
	}
	totalTestDurationInSeconds := totalDurationInMins * 60
	subTestDurationInSeconds := int((totalTestDurationInSeconds / numSubTests) + 1)
	subTestDuration := time.Duration(subTestDurationInSeconds) * time.Second
	return time.Now().Add(subTestDuration)
}

func retrieveAllTabs(ctx context.Context, tconn *chrome.TestConn, timeout time.Duration) ([]map[string]interface{}, error) {
	emptyQuery := map[string]interface{}{}

	// Get all tabs
	var tabs []map[string]interface{}
	err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := tconn.Call(ctx, &tabs, `tast.promisify(chrome.tabs.query)`, emptyQuery); err != nil {
			return testing.PollBreak(err)
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout})

	if err != nil {
		return nil, err
	}

	return tabs, nil
}

func focusTab(ctx context.Context, tconn *chrome.TestConn, tabs *[]map[string]interface{}, tabIndexWithinWindow int, timeout time.Duration) error {
	// Define parameters for API calls
	activateTabProperties := map[string]interface{}{
		"active": true,
	}

	// Find id of tab with positional index.
	tabID := int((*tabs)[tabIndexWithinWindow]["id"].(float64))

	// Switch to this tab as the active window
	err := testing.Poll(ctx, func(ctx context.Context) error {
		var tab map[string]interface{}
		if err := tconn.Call(ctx, &tab, `tast.promisify(chrome.tabs.update)`, tabID, activateTabProperties); err != nil {
			return testing.PollBreak(err)
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout})

	if err != nil {
		return err
	}

	return nil
}

func closeConnections(ctx context.Context, s *testing.State, conns []*chrome.Conn) {
	for _, c := range conns {
		if err := c.CloseTarget(ctx); err != nil {
			s.Error("Failed to close target: ", err)
		}
		if err := c.Close(); err != nil {
			s.Error("Failed to close the connection: ", err)
		}
	}
}

func testBody(ctx context.Context) error {
	// Lacros Specific Setup
	var tabOffset int
	if test.param.BrowserType == browser.TypeLacros {
		tabOffset = 1
	} else {
		tabOffset = 0
	}

	for _, data := range test.WebPages {
		const numPages = 7
		const numExtraPages = numPages - 1
		conns := make([]*chrome.Conn, 0, numPages)

		// Create the homepage of the site.
		firstPage, err := test.cs.NewConn(ctx, data.startURL)
		if err != nil {
			state.Fatalf("Failed to open %s: %v", data.startURL, err)
		}
		conns = append(conns, firstPage)

		// Find extra urls to navigate to
		urls, err := findAnchorURLs(ctx, firstPage, data.urlPattern, numExtraPages)
		if err != nil {
			state.Fatalf("Failed to get URLs for %s: %v", data.startURL, err)
		}

		// Open those found URLs as new tabs
		for _, url := range urls {
			newConnection, err := test.cs.NewConn(ctx, url)
			if err != nil {
				state.Fatalf("Failed to open the URL %s: %v", url, err)
			}
			conns = append(conns, newConnection)
		}

		// Ensure that all tabs are properly loaded before starting test.
		err = waitUntilAllTabsLoaded(ctx, test.browserTestConn, time.Minute)
		if err != nil {
			state.Log("Some tabs are still in loading state, but proceed the test: ", err)
		}

		currentTab := 0
		const tabSwitchTimeout = 20 * time.Second

		// Repeat x times to increase duration of test.
		// Note: Test runs for *approximately* CoreTestDurationInMins long.
		endTime := getEndTimeForTest(CoreTestDurationInMins, len(test.WebPages))
		if endTime.IsZero() {
			state.Fatalf("GetEndTimeForTest() received %d as numSubTests, which can't be less than 1. Test didn't run.", len(test.WebPages))
		}

		// Get all tabs
		tabs, err := retrieveAllTabs(ctx, test.browserTestConn, tabSwitchTimeout)
		if err != nil {
			state.Fatal("Failed to retrieve tabs from browser: ", err)
		}

		// Switch through tabs in a skip-order fashion.
		// Note: when skipSize = N-1, then the skip-order is 1,1,1,1 ... N times
		// Therefore i + r + 1 % N holds when 0 <= r < N-1
		for time.Now().Before(endTime) {
			for skipSize := 0; skipSize < len(conns)-1; skipSize++ {
				for i := 0; i < len(conns); i++ {
					inBrowserTabIndex := currentTab + tabOffset
					if err := focusTab(ctx, test.browserTestConn, &tabs, inBrowserTabIndex, tabSwitchTimeout); err != nil {
						state.Fatalf("Failed to switch to tab index %d. Error: %s", currentTab, err)
					}

					if err := webutil.WaitForRender(ctx, conns[currentTab], tabSwitchTimeout); err != nil {
						state.Fatal("Failed to wait for the tab to be visible: ", err)
					}

					currentTab = (currentTab + skipSize + 1) % len(conns)
				}
				currentTab = 0
			}
		}

		// Close previously opened tabs/window.
		closeConnections(ctx, state, conns)
	}

	return nil
}

// RunTest runs the setup, core part of the TabSwitchCUJ3 test, and cleanup.
func RunTest(ctx context.Context, s *testing.State) {
	// Perform initial test setup
	setupVariables, err := runSetup(ctx, s, s.Param().(TabSwitchParam))
	if err != nil {
		s.Fatalf("Failed to run setup: %s", err)
	}
	defer lacros.CloseLacros(ctx, test.l)

	// Singleton of all relevant variables to be used
	// throughout the test.
	test = setupVariables

	// Shorten context a bit to allow for cleanup.
	closeCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Second)
	defer cancel()
	defer test.recorder.Close(closeCtx)

	muteDevice(ctx, s)

	// Validation Specific Setup
	if test.param.Validation {
		validationHelper := cuj.NewTPSValidationHelper(closeCtx)
		if err := validationHelper.Stress(); err != nil {
			s.Fatal("Failed to stress: ", err)
		}
		defer func() {
			if err := validationHelper.Release(); err != nil {
				s.Fatal("Failed to release validationHelper: ", err)
			}
		}()
	}

	// Execute Test
	s.Run(ctx, "WebPage Tab Switching", func(ctx context.Context, s *testing.State) {
		recorderErr := test.recorder.Run(ctx, testBody)
		if recorderErr != nil {
			s.Fatal("Failed to conduct the test scenario, or collect the histogram data: ", recorderErr)
		}
	})

	// Write out values
	pv := perf.NewValues()
	if err = test.recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to report: ", err)
	}
	if err = pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to store values: ", err)
	}
}
