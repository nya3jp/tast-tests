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

	"chromiumos/tast/common/action"
	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	sim "chromiumos/tast/local/chrome/cuj/inputsimulations"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/event"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
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

	cr           *chrome.Chrome
	br           *browser.Browser
	closeBrowser func(context.Context) error
	tconn        *chrome.TestConn
	bTconn       *chrome.TestConn
	recorder     *cujrecorder.Recorder
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

	vars.bTconn, err = vars.br.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get browser TestAPIConn")
	}

	vars.recorder, err = cujrecorder.NewRecorder(ctx, vars.cr, vars.bTconn, nil, cujrecorder.RecorderOptions{})
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

	vars.tconn, err = vars.cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get ash-chrome test connection")
	}

	if err := vars.recorder.AddCommonMetrics(vars.tconn, vars.bTconn); err != nil {
		s.Fatal("Failed to add common metrics to the recorder: ", err)
	}

	vars.recorder.EnableTracing(s.OutDir(), s.DataPath(cujrecorder.SystemTraceConfigFile))

	if _, ok := s.Var("record"); ok {
		if err := vars.recorder.AddScreenRecorder(ctx, vars.tconn, s.TestName()); err != nil {
			s.Fatal("Failed to add screen recorder: ", err)
		}
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

func testBody(ctx context.Context, test *tabSwitchVariables) error {
	const (
		numPages         = 7
		tabSwitchTimeout = 20 * time.Second
	)

	info, err := display.GetPrimaryInfo(ctx, test.tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get the primary display info")
	}

	kw, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open the keyboard")
	}
	defer kw.Close()

	// Create a virtual mouse.
	mw, err := input.Mouse(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create a mouse")
	}
	defer mw.Close()

	ac := uiauto.New(test.tconn)

	for _, data := range test.webPages {
		conns := make([]*chrome.Conn, 0, numPages)

		// Create the homepage of the site.
		firstPage, err := test.br.NewConn(ctx, data.startURL)
		if err != nil {
			return errors.Wrapf(err, "failed to open %s", data.startURL)
		}
		conns = append(conns, firstPage)

		if test.param.BrowserType == browser.TypeLacros {
			if err := browser.CloseTabByTitle(ctx, test.bTconn, "New Tab"); err != nil {
				return errors.Wrap(err, `failed to close "New Tab" tab`)
			}
		}

		// Find extra urls to navigate to.
		urls, err := findAnchorURLs(ctx, firstPage, data.urlPattern, numPages-1)
		if err != nil {
			return errors.Wrapf(err, "failed to get URLs for %s", data.startURL)
		}

		// Open those found URLs as new tabs.
		for _, url := range urls {
			newConnection, err := test.br.NewConn(ctx, url)
			if err != nil {
				return errors.Wrapf(err, "failed to open the URL %s", url)
			}
			conns = append(conns, newConnection)
		}

		// Ensure that all tabs are properly loaded before starting test.
		if err := waitUntilAllTabsLoaded(ctx, test.bTconn, time.Minute); err != nil {
			testing.ContextLog(ctx, "Some tabs are still in loading state, but proceeding with the test: ", err)
		}

		// Repeat the test as many times as necessary to fulfill its time requirements.
		// e.g. If there are two windows that need to be tested sequentially, and the
		// total core test duration is 10 mins, each window will be tested for 5 mins.
		//
		// Note: Test runs for coreTestDuration minutes.
		if len(test.webPages) == 0 {
			return errors.New("test scenario does not specify any web pages")
		}

		testing.ContextLog(ctx, "Start switching tabs")

		// Switch through tabs in a skip-order fashion.
		// Note: when skipSize = N-1, then the skip-order is 1,1,1,1 ... N times
		// Therefore i + skipSize + 1 % N holds when 0 <= skipSize < N-1
		skipSize := 0
		i := 0
		currentTab := 0
		endTime := time.Now().Add(coreTestDuration/time.Duration(len(test.webPages)) + time.Second)
		for time.Now().Before(endTime) {
			tabToClick := nodewith.HasClass("TabIcon").Nth(currentTab)
			if err := action.Combine(
				"click on tab and move mouse back to the center of the display",
				ac.MouseMoveTo(tabToClick, 500*time.Millisecond),
				ac.LeftClick(tabToClick),
				mouse.Move(test.tconn, info.Bounds.CenterPoint(), 500*time.Millisecond),
			)(ctx); err != nil {
				return err
			}

			if err := webutil.WaitForQuiescence(ctx, conns[currentTab], tabSwitchTimeout); err != nil {
				return errors.Wrap(err, "failed to wait for the tab to quiesce")
			}

			for _, key := range []string{"Down", "Up"} {
				if err := sim.RepeatKeyPress(ctx, kw, key, 200*time.Millisecond, 3); err != nil {
					return errors.Wrapf(err, "failed to repeatedly press %s in between tab switches", key)
				}
			}
			for _, scrollDown := range []bool{true, false} {
				if err := sim.RepeatMouseScroll(ctx, mw, scrollDown, 50*time.Millisecond, 20); err != nil {
					return errors.Wrap(err, "failed to scroll in between tab switches")
				}
			}

			if err := ac.WithInterval(time.Second).WithTimeout(5*time.Second).WaitUntilNoEvent(nodewith.Root(), event.LocationChanged)(ctx); err != nil {
				testing.ContextLog(ctx, "Scroll animations haven't stabilized yet, continuing anyway: ", err)
			}

			if err := sim.RunDragMouseCycle(ctx, test.tconn, info); err != nil {
				return errors.Wrap(err, "failed to run the mouse drag cycle")
			}

			currentTab = (currentTab + skipSize + 1) % len(conns)

			// Once we have seen every tab, adjust the skipSize to
			// vary the tab visitation order.
			if i == len(conns)-1 {
				i = 0
				currentTab = 0
				skipSize = (skipSize + 1) % len(conns)
			} else {
				i++
			}
		}

		switch test.param.BrowserType {
		case browser.TypeLacros:
			if err := browser.ReplaceAllTabsWithSingleNewTab(ctx, test.bTconn); err != nil {
				return errors.Wrap(err, "failed to close all tabs and leave a single new tab open")
			}
		case browser.TypeAsh:
			if err := browser.CloseAllTabs(ctx, test.bTconn); err != nil {
				return errors.Wrap(err, "failed to close all tabs")
			}
		default:
			return errors.Errorf("unsupported browser type %v", test.param.BrowserType)
		}
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
		return testBody(ctx, setupVars)
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
