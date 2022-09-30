// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package tabswitchcuj contains the test code for TabSwitchCUJ. The test is
// extracted into this package to be shared between TabSwitchCUJRecorder and
// TabSwitchCUJ.
//
// Steps to update the test:
//  1. Make changes in this package.
//  2. "tast run $IP ui.TabSwitchCUJRecorder" to record the contents.
//     Look for the recorded wpr archive in /tmp/tab_switch_cuj.wprgo.
//  3. Update the recorded wpr archive to cloud storage under
//     gs://chromiumos-test-assets-public/tast/cros/ui/
//     It is recommended to add a date suffix to make it easier to change.
//  4. Update "tab_switch_cuj.wprgo.external" file under ui/data.
//  5. "tast run $IP ui.TabSwitchCUJ" locally to make sure tests works
//     with the new recorded contents.
//  6. Submit the changes here with updated external data reference.
package tabswitchcuj

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
)

const (
	// WPRArchiveName is used as the external file name of the wpr archive for
	// TabSwitchCuj and as the output filename under "/tmp" for
	// TabSwitchCujRecorder.
	WPRArchiveName = "tab_switch_cuj.wprgo"
)

// TabSwitchParam holds parameters of tab switch cuj test variations.
type TabSwitchParam struct {
	BrowserType browser.Type // Chrome type.
}

// tabSwitchVariables holds all the necessary variables used by the test.
type tabSwitchVariables struct {
	param    TabSwitchParam // Test Parameters
	webPages []webPage      // List of sites to visit

	cr              *chrome.Chrome
	br              *browser.Browser
	closeBrowser    func(context.Context) error
	browserTestConn *chrome.TestConn
	recorder        *cujrecorder.Recorder
}

// webPage holds the info used to visit new sites in the test.
type webPage struct {
	name       string // Display Name of the Website
	startURL   string // Base URL to the Website
	urlPattern string // RegExp Pattern to Open Relevant Links on the Website
}

// coreTestDuration is a minimum duration for the core part of the test.
// The actual test duration could be longer because of various setup.
const coreTestDuration = 10 * time.Minute

func runSetup(ctx context.Context, s *testing.State) (*tabSwitchVariables, error) {
	vars := tabSwitchVariables{
		param:    s.Param().(TabSwitchParam),
		webPages: getTestWebpages(),
	}

	switch vars.param.BrowserType {
	case browser.TypeAsh:
		vars.cr = s.PreValue().(chrome.HasChrome).Chrome()
	case browser.TypeLacros:
		vars.cr = s.FixtValue().(chrome.HasChrome).Chrome()
	default:
		return nil, errors.Errorf("unsupported browser type: %v", vars.param.BrowserType)
	}
	var err error
	vars.br, vars.closeBrowser, err = browserfixt.SetUp(ctx, vars.cr, vars.param.BrowserType)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open the browser")
	}

	vars.browserTestConn, err = vars.br.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get browser TestAPIConn")
	}

	vars.recorder, err = cujrecorder.NewRecorder(ctx, vars.cr, vars.browserTestConn, nil, cujrecorder.RecorderOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a recorder")
	}
	metricsSuccessfullyAdded := false
	defer func(ctx context.Context) {
		if metricsSuccessfullyAdded {
			return
		}
		vars.closeBrowser(ctx)
		vars.recorder.Close(ctx)
	}(ctx)

	var ashTestConn *chrome.TestConn
	if ashTestConn, err = vars.cr.TestAPIConn(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to get ash-chrome test connection")
	}

	if err := vars.recorder.AddCollectedMetrics(ashTestConn, vars.param.BrowserType, cujrecorder.AshCommonMetricConfigs()...); err != nil {
		return nil, errors.Wrap(err, "failed to add ash common metrics to recorder")
	}

	if err := vars.recorder.AddCollectedMetrics(vars.browserTestConn, vars.param.BrowserType, cujrecorder.BrowserCommonMetricConfigs()...); err != nil {
		return nil, errors.Wrap(err, "failed to add browser common metrics to recorder")
	}

	if err := vars.recorder.AddCollectedMetrics(vars.browserTestConn, vars.param.BrowserType, cujrecorder.AnyChromeCommonMetricConfigs()...); err != nil {
		return nil, errors.Wrap(err, "failed to add any chrome common metrics to recorder")
	}

	vars.recorder.EnableTracing(s.OutDir(), s.DataPath(cujrecorder.SystemTraceConfigFile))

	if _, ok := s.Var("record"); ok {
		if err := vars.recorder.AddScreenRecorder(ctx, ashTestConn, s.TestName()); err != nil {
			s.Fatal("Failed to add screen recorder: ", err)
		}
	}

	// The test runs for 10 minutes + time to open the browser. Thus,
	// take a screenshot every 2 minutes, with a max of 6 screenshots.
	if err := vars.recorder.AddScreenshotRecorder(ctx, 2*time.Minute, 6); err != nil {
		s.Fatal("Failed to add screenshot recorder: ", err)
	}

	metricsSuccessfullyAdded = true
	return &vars, nil
}

func getTestWebpages() []webPage {
	CNN := webPage{
		name:       "CNN",
		startURL:   "https://cnn.com",
		urlPattern: `^.*://www.cnn.com/\d{4}/\d{2}/\d{2}/`,
	}

	Reddit := webPage{
		name:       "Reddit",
		startURL:   "https://reddit.com",
		urlPattern: `^.*://www.reddit.com/r/[^/]+/comments/[^/]+/`,
	}

	return []webPage{CNN, Reddit}
}

func muteDevice(ctx context.Context, s *testing.State) error {
	// The custom variable for the developer to mute the device before the test,
	// so it doesn't make any noise when some of the visited pages play video.
	if _, ok := s.Var("mute"); !ok {
		return nil
	}

	kw, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open the keyboard")
	}
	defer kw.Close()

	topRow, err := input.KeyboardTopRowLayout(ctx, kw)
	if err != nil {
		return errors.Wrap(err, "failed to obtain the top-row layout")
	}
	if err = kw.Accel(ctx, topRow.VolumeMute); err != nil {
		return errors.Wrap(err, "failed to press mute key")
	}

	return nil
}

// findAnchorURLs returns the unique URLs of the anchors, which matches the pattern.
// If it finds more than limit, returns the first limit elements.
func findAnchorURLs(ctx context.Context, c *chrome.Conn, pattern string, limit int) ([]string, error) {
	var urls []string
	if err := c.Call(ctx, &urls, `(pattern, limit) => {
		const anchors = [...document.getElementsByTagName('A')];
		const founds = new Set();
		const results = [];
		const regexp = new RegExp(pattern);
		for (let i = 0; i < anchors.length && results.length < limit; i++) {
		  const href = new URL(anchors[i].href).toString();
		  if (founds.has(href)) {
		    continue;
		  }
		  founds.add(href);
		  if (regexp.test(href)) {
		    results.push(href);
		  }
		}
		return results;
	}`, pattern, limit); err != nil {
		return nil, err
	}
	if len(urls) == 0 {
		return nil, errors.New("no urls found")
	}
	return urls, nil
}

func waitUntilAllTabsLoaded(ctx context.Context, tconn *chrome.TestConn, timeout time.Duration) error {
	query := map[string]interface{}{
		"status":        "loading",
		"currentWindow": true,
	}
	return testing.Poll(ctx, func(ctx context.Context) error {
		var tabs []map[string]interface{}
		if err := tconn.Call(ctx, &tabs, `tast.promisify(chrome.tabs.query)`, query); err != nil {
			return testing.PollBreak(err)
		}
		if len(tabs) != 0 {
			return errors.Errorf("still %d tabs are loading", len(tabs))
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout})
}

func retrieveAllTabs(ctx context.Context, tconn *chrome.TestConn, timeout time.Duration) ([]map[string]interface{}, error) {
	emptyQuery := map[string]interface{}{}

	// Get all tabs
	var tabs []map[string]interface{}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	err := tconn.Call(ctx, &tabs, `tast.promisify(chrome.tabs.query)`, emptyQuery)
	return tabs, err
}

func focusTab(ctx context.Context, tconn *chrome.TestConn, tabs *[]map[string]interface{}, tabIndexWithinWindow int, timeout time.Duration) error {
	// Define parameters for API calls
	activateTabProperties := map[string]interface{}{
		"active": true,
	}

	// Find id of tab with positional index.
	tabID := int((*tabs)[tabIndexWithinWindow]["id"].(float64))

	// Switch to this tab as the active window
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return tconn.Call(ctx, nil, `tast.promisify(chrome.tabs.update)`, tabID, activateTabProperties)
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

func testBody(ctx context.Context, s *testing.State, test *tabSwitchVariables) error {
	// Lacros Specific Setup
	var tabOffset int
	switch test.param.BrowserType {
	case browser.TypeLacros:
		tabOffset = 1
	case browser.TypeAsh:
		tabOffset = 0
	default:
		return errors.Errorf("unsupported browser type: %v", test.param.BrowserType)
	}

	for _, data := range test.webPages {
		const numPages = 7
		const numExtraPages = numPages - 1
		conns := make([]*chrome.Conn, 0, numPages)

		// Create the homepage of the site.
		firstPage, err := test.br.NewConn(ctx, data.startURL)
		if err != nil {
			return errors.Wrapf(err, "failed to open %s", data.startURL)
		}
		conns = append(conns, firstPage)

		// Find extra urls to navigate to
		urls, err := findAnchorURLs(ctx, firstPage, data.urlPattern, numExtraPages)
		if err != nil {
			return errors.Wrapf(err, "failed to get URLs for %s", data.startURL)
		}

		// Open those found URLs as new tabs
		for _, url := range urls {
			newConnection, err := test.br.NewConn(ctx, url)
			if err != nil {
				return errors.Wrapf(err, "failed to open the URL %s", url)
			}
			conns = append(conns, newConnection)
		}

		// Ensure that all tabs are properly loaded before starting test.
		if err := waitUntilAllTabsLoaded(ctx, test.browserTestConn, time.Minute); err != nil {
			s.Log("Some tabs are still in loading state, but proceeding with the test: ", err)
		}

		currentTab := 0
		const tabSwitchTimeout = 20 * time.Second

		// Repeat the test as many times as necessary to fulfill its time requirements.
		// e.g. If there are two windows that need to be tested sequentially, and the
		// total core test duration is 10 mins, each window will be tested for 5 mins.
		//
		// Note: Test runs for *approximately* coreTestDuration minutes.
		if len(test.webPages) == 0 {
			return errors.New("test scenario does not specify any web pages")
		}
		endTime := time.Now().Add(coreTestDuration/time.Duration(len(test.webPages)) + time.Second)

		// Get all tabs
		tabs, err := retrieveAllTabs(ctx, test.browserTestConn, tabSwitchTimeout)
		if err != nil {
			return errors.Wrap(err, "failed to retrieve tabs from browser")
		}

		// Switch through tabs in a skip-order fashion.
		// Note: when skipSize = N-1, then the skip-order is 1,1,1,1 ... N times
		// Therefore i + skipSize + 1 % N holds when 0 <= skipSize < N-1
		for time.Now().Before(endTime) {
			for skipSize := range conns {
				for range conns {
					inBrowserTabIndex := currentTab + tabOffset
					if err := focusTab(ctx, test.browserTestConn, &tabs, inBrowserTabIndex, tabSwitchTimeout); err != nil {
						return errors.Wrapf(err, "failed to switch to tab index %d", currentTab)
					}

					if err := webutil.WaitForRender(ctx, conns[currentTab], tabSwitchTimeout); err != nil {
						return errors.Wrap(err, "failed to wait for the tab to be visible")
					}

					currentTab = (currentTab + skipSize + 1) % len(conns)
				}
				currentTab = 0
			}
		}

		// Close previously opened tabs/window.
		closeConnections(ctx, s, conns)
	}

	return nil
}

// Run runs the setup, core part of the TabSwitchCUJ test, and cleanup.
func Run(ctx context.Context, s *testing.State) {
	// Reserve time for cleanup
	closeCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 2*time.Second)
	defer cancel()

	// Perform initial test setup
	setupVars, err := runSetup(ctx, s)
	if err != nil {
		s.Fatal("Failed to run setup: ", err)
	}
	defer setupVars.closeBrowser(closeCtx)
	defer setupVars.recorder.Close(closeCtx)

	if err := muteDevice(ctx, s); err != nil {
		s.Log("(non-error) Failed to mute device: ", err)
	}

	// Execute Test
	if err := setupVars.recorder.Run(ctx, func(ctx context.Context) error {
		return testBody(ctx, s, setupVars)
	}); err != nil {
		s.Fatal("Failed to conduct the test scenario, or collect the histogram data: ", err)
	}

	// Write out values
	pv := perf.NewValues()
	if err := setupVars.recorder.Record(ctx, pv); err != nil {
		s.Fatal("Failed to report: ", err)
	}
	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to store values: ", err)
	}
}
