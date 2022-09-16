// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package tabswitchcuj contains the test code for TabSwitchCUJ. The test is
// extracted into this package to be shared between TabSwitchCUJRecorder and
// TabSwitchCUJ.
package tabswitchcuj

import (
	"context"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"github.com/mafredri/cdp/protocol/target"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/metrics"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
)

const (
	shortUITimeout = 3 * time.Second

	tabSwitchTimeout = 2 * time.Minute
	clickLinkTimeout = 1 * time.Minute

	replayPageLoadingTimeout    = 2 * time.Minute
	recordingPageLoadingTimeout = 5 * time.Minute
)

// pageLoadingTimeout returns the timeout value when waiting for a page being loaded.
func pageLoadingTimeout(caseLevel Level) time.Duration {
	// In record mode, give more time to loading to ensure web content is fully recorded.
	if caseLevel == Record {
		return recordingPageLoadingTimeout
	}
	return replayPageLoadingTimeout
}

// Level indicate how intensive of this test case is going to execute.
type Level uint8

// Level indicate how intensive of this test case is going to execute.
//
// Basic is the level to use to run this case in basic level
// Plus is the level to use to run this case in plus level
// Premium is the level to use to run this case in basic level
// Record is the level to use to run this case in *record mode*
const (
	Basic Level = iota
	Plus
	Premium
	Record
)

// webType define all web site is involved in this test case
type webType string

const (
	wikipedia    webType = "Wikipedia"
	reddit       webType = "Reddit"
	medium       webType = "Medium"
	yahooNews    webType = "YahooNews"
	yahooFinance webType = "YahooFinance"
	cnn          webType = "CNN"
	espn         webType = "ESPN"
	hulu         webType = "Hulu"
	pinterest    webType = "Pinterest"
	youtube      webType = "Youtube"
	netflix      webType = "Netflix"
)

// webPageInfo records a Chrome page's information, including the current browsing page
// and url links (in patterns) for page navigation.
type webPageInfo struct {
	level   Level   // the test level of this link will be used for. Only used for generating targets
	webName webType // current page's website name
	// contentPatterns holds the patterns of the url links embedded in the web page. During
	// tab switch, we find the url of the given pattern in the current page and click it.
	// Links can be clicked back and forth in case multiple rounds of tab switch are executed.
	contentPatterns []string
}

func newPageInfo(level Level, web webType, patterns ...string) *webPageInfo {
	if len(patterns) < 2 {
		panic("Invalid configuration of webPageInfo")
	}

	return &webPageInfo{
		level:           level,
		webName:         web,
		contentPatterns: patterns,
	}
}

// chromeTab holds the information of a Chrome browser tab.
type chromeTab struct {
	conn           *chrome.Conn
	pageInfo       *webPageInfo // static information (e.g. the type of website being visited, which contents to search to click) of this tab
	url            string       // current url of the website being visited
	currentPattern int          // the index of current page's corresponding content pattern within pageInfo
}

var (
	errHTMLelementNotFound = errors.New("failed to find HTML element on page")
	errNotClickAndNavigate = errors.New("has not clicked HTML link and navigate")
)

func (tab *chromeTab) searchElementWithPatternAndClick(ctx context.Context, patterns []string) (int, error) {
	if err := tab.conn.Eval(ctx, "window.location.href", &tab.url); err != nil {
		return 0, errors.Wrap(err, "failed to get URL")
	}
	testing.ContextLogf(ctx, "Current URL: %q", tab.url)

	const (
		statusDone            = -1
		statusElementNotFound = -2
	)

	links := `'` + strings.Join(patterns, `', '`) + `'`
	script := fmt.Sprintf(`() => {
		if (window.location.href !== '%s') {
			return %d;
		}
		const elements = [%s];
		for (let i = 0; i < elements.length; i++) {
			const pattern = elements[i];
			const name = "a[href*='" + pattern + "']";
			const els = document.querySelectorAll(name);
			if (els.length === 0) {
				continue;
			} else {
				els[0].click();
				return i;
			}
		}
		return %d;
	}`, tab.url, statusDone, links, statusElementNotFound)

	pattern := ""
	foundPatternIndex := 0
	timeout := 90 * time.Second
	numberOfRetryForJsError := 5
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		status := 0
		if err := tab.conn.Call(ctx, &status, script); err != nil {
			// [cienet private] retry JS script evaluation for error 211
			if numberOfRetryForJsError <= 0 {
				return testing.PollBreak(errors.Wrap(err, "failed to execute JavaScript query to click HTML link to navigate"))
			}
			testing.ContextLog(ctx, "Retry JavaScript query on error")
			numberOfRetryForJsError--
		}
		if status == statusElementNotFound {
			return testing.PollBreak(errHTMLelementNotFound)
		}
		if status == statusDone {
			return errNotClickAndNavigate
		}
		// When "status" is greater than or equal to 0, returns an integer representing the index of the element that can be found.
		if status > 0 {
			testing.ContextLog(ctx, "Missing patterns: ", patterns[0:status])
		}
		foundPatternIndex = status
		pattern = patterns[foundPatternIndex]
		return nil
	}, &testing.PollOptions{Timeout: timeout, Interval: 100 * time.Millisecond}); err != nil {
		return 0, errors.Wrapf(err, "failed to click HTML element and navigate within %v", timeout)
	}
	if err := tab.conn.Eval(ctx, "window.location.href", &tab.url); err != nil {
		return 0, errors.Wrap(err, "failed to get URL")
	}
	testing.ContextLogf(ctx, "HTML element clicked [%s], page navigates to: %q", pattern, tab.url)
	tab.url = strings.TrimSuffix(tab.url, "/")

	return foundPatternIndex, nil
}

func (tab *chromeTab) clickAnchor(ctx context.Context, timeout time.Duration, tconn *chrome.TestConn) error {
	const (
		histogramName     = "PageLoad.PaintTiming.NavigationToLargestContentfulPaint2"
		histogramWaitTime = time.Second
	)
	p := tab.currentPattern
	pn := p + 1
	numPatterns := len(tab.pageInfo.contentPatterns)
	if pn == numPatterns {
		pn = 0
	}

	if err := webutil.WaitForQuiescence(ctx, tab.conn, timeout); err != nil {
		// It has been seen that content sites such as ESPN sometimes can take minutes to reach
		// quiescence on DUTs. When this occurred, it can be seen from screenshots that the UI has
		// actually loaded but background tasks prevented the site to reach quiescence. Therefore,
		// logic is added here to check whether the site has loaded. If the site has loaded, i.e.,
		// the site readyState is not "loading", no error will be returned here.
		if err := tab.conn.WaitForExprFailOnErrWithTimeout(ctx, `document.readyState === "interactive" || document.readyState === "complete"`, 3*time.Second); err != nil {
			return errors.Wrapf(err, "failed to wait for tab to load within %v before clicking anchor", timeout)
		}
		testing.ContextLogf(ctx, "%s could not reach quiescence within %v, but document state has passed loading", tab.url, timeout)
	}

	h1, err := metrics.GetHistogram(ctx, tconn, histogramName)
	if err != nil {
		return errors.Wrap(err, "failed to get histogram")
	}
	testing.ContextLog(ctx, "Got LCP2 histogram: ", h1)

	patternsToFind := append(tab.pageInfo.contentPatterns[pn:numPatterns], tab.pageInfo.contentPatterns[0:p]...)
	foundPatternIndex, err := tab.searchElementWithPatternAndClick(ctx, patternsToFind)
	if err != nil {
		// Check whether the failure to search and click pattern was due to issues on the content site.
		if contentSiteErr := contentSiteUnavailable(ctx, tconn); contentSiteErr != nil {
			return errors.Wrapf(contentSiteErr, "failed to show content on page %s", tab.url)
		}
		return errors.Wrapf(err, "failed to click anchor on page %s", tab.url)
	}

	testing.ContextLogf(ctx, "Waiting for %v histogram update", histogramName)
	h2, err := metrics.WaitForHistogramUpdate(ctx, tconn, histogramName, h1, histogramWaitTime)
	if err != nil {
		testing.ContextLog(ctx, "Failed to get histogram update: ", err)
	} else {
		testing.ContextLog(ctx, "Got LCP2 histogram update: ", h2)
	}

	tab.currentPattern = (pn + foundPatternIndex) % numPatterns
	return nil
}

func contentSiteUnavailable(ctx context.Context, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn)

	errorMessages := []string{
		"503 Service Temporarily Unavailable",
		"504 Gateway Time-out",
		"HTTP ERROR 404",
		"Our CDN was unable to reach our servers",
		"Apologies, but something went wrong on our end.",
		"This site can’t provide a secure connection",
		"This site can’t be reached",
	}

	for _, m := range errorMessages {
		node := nodewith.Name(m).Role(role.StaticText)
		if err := ui.Exists(node)(ctx); err == nil {
			return errors.Errorf("content site error - %s", m)
		}
	}
	return nil
}

func (tab *chromeTab) close(ctx context.Context) error {
	if tab.conn == nil {
		return nil
	}
	if err := tab.conn.CloseTarget(ctx); err != nil {
		return errors.Wrap(err, "failed to close tab")
	}
	if err := tab.conn.Close(); err != nil {
		return errors.Wrap(err, "failed to close tab connection")
	}
	tab.conn = nil
	return nil
}

// reconnect checks if the tab connection is still alive and reconnect if it isn't.
//
// It has been observed that tabs can be discarded when DUT runs low on memory, causing unexpected behaviors
// and ultimately test failures. For details, please refer to b/184571798.
func (tab *chromeTab) reconnect(ctx context.Context, cr *chrome.Chrome) error {
	var url string
	// Verify if the tab connection is still usable.
	err := tab.conn.Eval(ctx, "window.location.href", &url)
	if err == nil || !tabDiscarded(err) {
		testing.ContextLog(ctx, "The connection is alive and therefore no need to reconnect")
		return nil
	}
	testing.ContextLog(ctx, "Tab has been discarded/killed, reconnecting")

	matcher := func(t *target.Info) bool {
		actualURL := strings.Split(strings.TrimSuffix(t.URL, "/"), "?")[0] // Ignore all possible parameters.
		expectedURL := strings.Split(strings.TrimSuffix(tab.url, "/"), "?")[0]
		return t.Type == "page" && actualURL == expectedURL
	}

	if tab.conn, err = cr.NewConnForTarget(ctx, matcher); err != nil {
		return errors.Wrapf(err, "failed to reconnect to target %q", tab.url)
	}

	testing.ContextLogf(ctx, "Target (tab: [%s][%s]) was detached but successfully reconnected", tab.pageInfo.webName, tab.url)
	return nil
}

// tabDiscarded returns true if the input error is caused by tab discarding.
func tabDiscarded(err error) bool {
	// Discarded tab will return errors with the following pattern (b/184571798).
	p := regexp.MustCompile(`rpcc: the connection is closing: session: detach failed for session [0-9A-F]{32}: cdp.Target: DetachFromTarget: rpc error: No session with given id`)
	return p.MatchString(err.Error())
}

// chromeWindow is the struct for Chrome browser window. It holds multiple tabs.
type chromeWindow struct {
	tabs []*chromeTab
}

var allTargets = []struct {
	url  string
	info *webPageInfo
}{
	{cuj.WikipediaMainURL, newPageInfo(Basic, wikipedia, `/Main_Page`, `/Wikipedia:Contents`)},
	{cuj.WikipediaCurrentEventsURL, newPageInfo(Basic, wikipedia, `/Portal:Current_events`, `/Special:Random`)},
	{cuj.WikipediaAboutURL, newPageInfo(Basic, wikipedia, `/Wikipedia:About`, `/Wikipedia:Contact_us`)},
	{cuj.WikipediaHelpURL, newPageInfo(Plus, wikipedia, `/Help:Contents`, `/Help:Introduction`)},
	{cuj.WikipediaCommunityURL, newPageInfo(Plus, wikipedia, `/Wikipedia:Community_portal`, `/Special:RecentChanges`)},
	{cuj.WikipediaContributionURL, newPageInfo(Premium, wikipedia, `/Help:User_contributions`, `/Wikipedia`)},

	{cuj.RedditWallstreetURL, newPageInfo(Basic, reddit, `/r/wallstreetbets/hot/`, `/r/wallstreetbets/new/`)},
	{cuj.RedditTechNewsURL, newPageInfo(Basic, reddit, `/r/technews/hot/`, `/r/technews/new/`)},
	{cuj.RedditOlympicsURL, newPageInfo(Basic, reddit, `/r/olympics/hot/`, `/r/olympics/new/`)},
	{cuj.RedditProgrammingURL, newPageInfo(Plus, reddit, `/r/programming/hot/`, `/r/programming/new/`)},
	{cuj.RedditAppleURL, newPageInfo(Plus, reddit, `/r/apple/hot/`, `/r/apple/new/`)},
	{cuj.RedditBrooklynURL, newPageInfo(Premium, reddit, `/r/brooklynninenine/hot/`, `/r/brooklynninenine/new/`)},

	// Since "Medium" sites change content frequently, add an alternate tag link pattern.
	{cuj.MediumBusinessURL, newPageInfo(Basic, medium, `/business`, `/economy`, `/money`, `/marketing`)},
	{cuj.MediumStartupURL, newPageInfo(Basic, medium, `/startup`, `/leadership`, `/marketing`, `/business`)},
	{cuj.MediumWorkURL, newPageInfo(Plus, medium, `/work`, `/productivity`, `/careers`, `/business`)},
	{cuj.MediumSoftwareURL, newPageInfo(Premium, medium, `/software-engineering`, `/programming`, `/coding`, `/technology`)},
	{cuj.MediumAIURL, newPageInfo(Premium, medium, `/artificial-intelligence`, `/data-science`, `/software-engineering`, `/programming`)},

	// Since "Yahoo" sites change content frequently, add an alternate tag link pattern.
	{cuj.YahooUsURL, newPageInfo(Basic, yahooNews, `/us/`, `/politics/`, `/world/`)},
	{cuj.YahooWorldURL, newPageInfo(Basic, yahooNews, `/world/`, `/coronavirus/`, `/health/`)},
	{cuj.YahooScienceURL, newPageInfo(Plus, yahooNews, `/science/`, `/originals/`, `/us/`)},
	{cuj.YahooFinanaceWatchlistURL, newPageInfo(Premium, yahooFinance, `/watchlists/`, `/news/`)},

	{cuj.CnnWorldURL, newPageInfo(Plus, cnn, `/world`, `/africa`)},
	{cuj.CnnAmericasURL, newPageInfo(Plus, cnn, `/americas`, `/asia`)},
	{cuj.CnnAustraliaURL, newPageInfo(Plus, cnn, `/australia`, `/china`)},
	{cuj.CnnEuropeURL, newPageInfo(Premium, cnn, `/europe`, `/india`)},
	{cuj.CnnMiddleEastURL, newPageInfo(Premium, cnn, `/middle-east`, `/uk`)},

	{cuj.EspnNflURL, newPageInfo(Plus, espn, `/nfl/scoreboard`, `/nfl/schedule`)},
	{cuj.EspnNbaURL, newPageInfo(Plus, espn, `/nba/scoreboard`, `/nba/schedule`)},
	{cuj.EspnCollegeBasketballURL, newPageInfo(Plus, espn, `/mens-college-basketball/scoreboard`, `/mens-college-basketball/schedule`)},
	{cuj.EspnTennisURL, newPageInfo(Premium, espn, `/tennis/dailyResults`, `/tennis/schedule`)},
	{cuj.EspnSoccerURL, newPageInfo(Premium, espn, `/soccer/scoreboard`, `/soccer/schedule`)},

	{cuj.HuluMoviesURL, newPageInfo(Plus, hulu, `/hub/movies`, `/hub/originals`)},
	{cuj.HuluKidsURL, newPageInfo(Premium, hulu, `/hub/kids`, `/hub/networks`)},

	{cuj.PinterestURL, newPageInfo(Plus, pinterest, `/ideas/`, `/ideas/holidays/910319220330/`)},

	{cuj.NetflixURL, newPageInfo(Premium, netflix, `/en`, `/en/legal/termsofuse`)},

	{cuj.YoutubeURL, newPageInfo(Premium, youtube, `/`, `/feed/explore`)},
}

// generateTabSwitchTargets sets all web targets according to the input Level.
func generateTabSwitchTargets(caseLevel Level) ([]*chromeWindow, error) {
	winNum := 1
	tabNum := 0
	switch caseLevel {
	case Basic:
		winNum = 2
		tabNum = 5
	case Plus:
		winNum = 4
		tabNum = 6
	case Premium, Record:
		winNum = 4
		tabNum = 9
	}

	var targets []struct {
		url  string
		info *webPageInfo
	}

	for _, tgt := range allTargets {
		if tgt.info.level <= caseLevel {
			targets = append(targets, tgt)
		}
	}
	if len(targets) < winNum*tabNum {
		return nil, errors.New("no enough web page targets to construct tabs")
	}
	// Shuffle the URLs to random order.
	rand := rand.New(rand.NewSource(1))
	rand.Shuffle(len(targets), func(i, j int) { targets[i], targets[j] = targets[j], targets[i] })

	// If even-numbered tabs are Wikipedia or Yahoo tabs, swap to odd-numbered tabs.
	j := 0
	isOddTab := func(idx int) bool {
		return (idx%tabNum)%2 == 0
	}
	for i := range targets {
		if !isOddTab(i) && (targets[i].info.webName == wikipedia || targets[i].info.webName == yahooNews) {
			for {
				if j >= len(targets) {
					break
				}
				if isOddTab(j) {
					webName := targets[j].info.webName
					if webName != wikipedia && webName != yahooNews {
						targets[i], targets[j] = targets[j], targets[i]
						j++
						break
					}
				}
				j++
			}
		}
	}
	idx := 0
	windows := make([]*chromeWindow, winNum)
	for i := range windows {
		tabs := make([]*chromeTab, tabNum)
		for j := range tabs {
			tabs[j] = &chromeTab{conn: nil, pageInfo: targets[idx].info, url: targets[idx].url, currentPattern: 0}
			idx++
		}
		windows[i] = &chromeWindow{tabs: tabs}
	}

	return windows, nil
}

func closeAllTabs(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, windows []*chromeWindow) error {
	// Close all tabs normally.
	failed := 0
	for _, window := range windows {
		for _, tab := range window.tabs {
			if err := tab.close(ctx); err != nil {
				failed++
			}
		}
	}

	if failed == 0 {
		// All tabs have been closed.
		return nil
	}
	testing.ContextLogf(ctx, "Failed to close %d tab(s), which could have been detached; tring directly close Chrome window", failed)
	return cuj.CloseChrome(ctx, tconn)
}

// Run2 runs the TabSwitchCUJ test. It is invoked by TabSwitchCujRecorder2 to
// record web contents via WPR and invoked by TabSwitchCUJ2 to execute the tests
// from the recorded contents. Additional actions will be executed in each tab.
func Run2(ctx context.Context, s *testing.State, cr *chrome.Chrome, caseLevel Level, isTablet bool, bt browser.Type) {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API, error: ", err)
	}

	// Shorten the context to cleanup crastestclient and resume battery charging.
	cleanUpCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	if _, ok := s.Var("spera.cuj_mute"); ok {
		if err := crastestclient.Mute(ctx); err != nil {
			s.Fatal("Failed to mute: ", err)
		}
		defer crastestclient.Unmute(cleanUpCtx)
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

	windows, err := generateTabSwitchTargets(caseLevel)
	if err != nil {
		s.Fatal("Failed to generate tab targets: ", err)
	}

	var tsAction cuj.UIActionHandler
	if isTablet {
		if tsAction, err = cuj.NewTabletActionHandler(ctx, tconn); err != nil {
			s.Fatal("Failed to create tablet action handler: ", err)
		}
	} else {
		if tsAction, err = cuj.NewClamshellActionHandler(ctx, tconn); err != nil {
			s.Fatal("Failed to create clamshell action handler: ", err)
		}
	}
	defer tsAction.Close()

	timeTabsOpenStart := time.Now()
	// Launch browser and track the elapsed time.
	l, browserStartTime, err := cuj.GetBrowserStartTime(ctx, tconn, true, isTablet, bt)
	if err != nil {
		s.Fatal("Failed to launch Chrome: ", err)
	}
	if l != nil {
		defer l.Close(ctx)
	}
	s.Log("Browser start ms: ", browserStartTime)
	br := cr.Browser()
	var bTconn *chrome.TestConn
	if l != nil {
		br = l.Browser()
		bTconn, err = l.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to get lacros test API Conn: ", err)
		}
	}
	defer func(ctx context.Context) {
		// To make debug easier, if something goes wrong, take screenshot before tabs are closed.
		faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
		faillog.SaveScreenshotOnError(ctx, cr, s.OutDir(), s.HasError)
		if err := closeAllTabs(ctx, cr, tconn, windows); err != nil {
			s.Error("Failed to cleanup: ", err)
		}
	}(ctx)

	pv := perf.NewValues()
	pv.Set(perf.Metric{
		Name:      "Browser.StartTime",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, float64(browserStartTime.Milliseconds()))

	// Open all windows and tabs.
	if err := openAllWindowsAndTabs(ctx, br, &windows, tsAction, caseLevel); err != nil {
		s.Fatal("Failed to open targets for tab switch: ", err)
	}
	// Maximize all windows to ensure a consistent state.
	if err := ash.ForEachWindow(ctx, tconn, func(w *ash.Window) error {
		return ash.SetWindowStateAndWait(ctx, tconn, w.ID, ash.WindowStateMaximized)
	}); err != nil {
		s.Fatal("Failed to maximize windows: ", err)
	}

	// Total time used from beginning to load all pages.
	timeElapsed := time.Since(timeTabsOpenStart)
	s.Log("All tabs opened Elapsed: ", timeElapsed)

	pv.Set(perf.Metric{
		Name:      "TabSwitchCUJ.ElapsedTime",
		Unit:      "ms",
		Direction: perf.SmallerIsBetter,
	}, float64(timeElapsed.Milliseconds()))

	// Shorten the context to cleanup recorder.
	cleanUpRecorderCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	options := cujrecorder.NewPerformanceCUJOptions()
	recorder, err := cujrecorder.NewRecorder(ctx, cr, bTconn, nil, options)
	if err != nil {
		s.Fatal("Failed to create a recorder, error: ", err)
	}
	defer recorder.Close(cleanUpRecorderCtx)
	if err := cuj.AddPerformanceCUJMetrics(tconn, bTconn, recorder); err != nil {
		s.Fatal("Failed to add metrics to recorder: ", err)
	}
	if collect, ok := s.Var("spera.collectTrace"); ok && collect == "enable" {
		recorder.EnableTracing(s.OutDir(), s.DataPath(cujrecorder.SystemTraceConfigFile))
	}

	// Shorten context a bit to allow for cleanup if Run fails.
	shorterCtx, cancel := ctxutil.Shorten(ctx, 3*time.Second)
	defer cancel()

	if err = recorder.Run(shorterCtx, func(ctx context.Context) error {
		return tabSwitchAction(ctx, cr, tconn, &windows, tsAction, caseLevel)
	}); err != nil {
		s.Fatal("Failed to execute tab switch action: ", err)
	}

	// Use a short timeout value so it can return fast in case of failure.
	recordCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if err := recorder.Record(recordCtx, pv); err != nil {
		s.Fatal("Failed to report, error: ", err)
	}
	if err = pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to store values, error: ", err)
	}
	if err := recorder.SaveHistograms(s.OutDir()); err != nil {
		s.Error("Failed to save histogram raw data: ", err)
	}
}

func openAllWindowsAndTabs(ctx context.Context, br *browser.Browser, targets *[]*chromeWindow, tsAction cuj.UIActionHandler, caseLevel Level) (err error) {
	windows := (*targets)
	plTimeout := pageLoadingTimeout(caseLevel)
	for idxWindow, window := range windows {
		for idxTab, tab := range window.tabs {
			testing.ContextLogf(ctx, "Opening window %d, tab %d", idxWindow+1, idxTab+1)

			if tab.conn, err = tsAction.NewChromeTab(ctx, br, tab.url, idxTab == 0); err != nil {
				return errors.Wrap(err, "failed to create new Chrome tab")
			}

			if err := webutil.WaitForRender(ctx, tab.conn, plTimeout); err != nil {
				return errors.Wrap(err, "failed to wait for render to finish")
			}

			// In replay mode, user won't be able to know whether the page is quiescence or not,
			// and it is not necessary to wait for quiescence in replay mode.
			// In record mode, needs to wait for quiescence to properly record web content.
			if caseLevel == Record {
				if err := webutil.WaitForQuiescence(ctx, tab.conn, plTimeout); err != nil {
					return errors.Wrapf(err, "failed to wait for tab to achieve quiescence within %v", plTimeout)
				}
			}
		}
	}

	return nil
}

func tabSwitchAction(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, targets *[]*chromeWindow, tsAction cuj.UIActionHandler, caseLevel Level) error {
	windows := (*targets)
	scrollActions := tsAction.ScrollChromePage(ctx)
	plTimeout := pageLoadingTimeout(caseLevel)

	chromeApp, err := apps.PrimaryBrowser(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to find the Chrome app")
	}

	ui := uiauto.New(tconn)
	for idx, window := range windows {
		testing.ContextLogf(ctx, "Switching to window #%d", idx+1)
		if err := tsAction.SwitchToAppWindowByIndex(chromeApp.Name, idx)(ctx); err != nil {
			return errors.Wrap(err, "failed to switch window")
		}

		tabTotalNum := len(window.tabs)
		for tabIdx := 0; tabIdx < tabTotalNum; tabIdx++ {
			testing.ContextLogf(ctx, "Switching tab to window %d, tab %d", idx+1, tabIdx+1)

			tab := window.tabs[tabIdx]
			if tab.pageInfo.webName == reddit || tab.pageInfo.webName == youtube {
				notificationsDialog := nodewith.NameContaining("Show notifications").ClassName("RootView").Role(role.AlertDialog)
				allowButton := nodewith.Name("Allow").Role(role.Button).Ancestor(notificationsDialog)
				if err := uiauto.IfSuccessThen(
					ui.WithTimeout(shortUITimeout).WaitUntilExists(notificationsDialog),
					tsAction.Click(allowButton),
				)(ctx); err != nil {
					return errors.Wrap(err, "failed to close alert dialog")
				}
			}

			if err := tsAction.SwitchToChromeTabByIndex(tabIdx)(ctx); err != nil {
				return errors.Wrap(err, "failed to switch tab")
			}

			// Test the tab connection and reconnect if necessary. This is necessary for
			// discarded tabs due to OOM issue.
			// After tab switching, the current focused tab should be active again so
			// reconnect should succeed.
			if err := tab.reconnect(ctx, cr); err != nil {
				return errors.Wrap(err, "cdp connection is invalid and failed to reconnect")
			}

			timeStart := time.Now()
			if err := webutil.WaitForRender(ctx, tab.conn, tabSwitchTimeout); err != nil {
				testing.ContextLog(ctx, "WaitForRender failed. Reconnect and retry")
				if err := tab.reconnect(ctx, cr); err != nil {
					return errors.Wrap(err, "failed to reconnect cdp connection")
				}
				if err := webutil.WaitForRender(ctx, tab.conn, tabSwitchTimeout); err != nil {
					return errors.Wrap(err, "failed to wait for render on the second try after reconnect")
				}
			}
			renderTime := time.Since(timeStart)
			// Debugging purpose message, to observe which tab takes unusual long time to render.
			testing.ContextLog(ctx, "Tab rendering time after switching: ", renderTime)
			if caseLevel == Record {
				if err := webutil.WaitForQuiescence(ctx, tab.conn, plTimeout); err != nil {
					return errors.Wrapf(err, "failed to wait for tab to achieve quiescence within %v", plTimeout)
				}
				quiescenceTime := time.Now().Sub(timeStart)
				// Debugging purpose message, to observe which tab takes unusual long time to quiescence
				testing.ContextLog(ctx, "Tab quiescence time after switching: ", quiescenceTime)
			}

			// In case of lose connection to the tab, need to update the URL to reconnect to it
			if err := tab.conn.Eval(ctx, "window.location.href", &tab.url); err != nil {
				return errors.Wrap(err, "failed update the URL of tab")
			}

			// To reduce total execution time of this test case,
			// these specific websites has been chosen to do scroll actions as per requirement.
			if tab.pageInfo.webName == wikipedia || tab.pageInfo.webName == hulu || tab.pageInfo.webName == youtube {
				for _, act := range scrollActions {
					if err := act(ctx); err != nil {
						return errors.Wrap(err, "failed to execute action")
					}
					// Make sure the whole web content is recorded only under Recording.
					if caseLevel == Record {
						if err := webutil.WaitForRender(ctx, tab.conn, tabSwitchTimeout); err != nil {
							return errors.Wrap(err, "failed to wait for render to finish after scroll")
						}
						if err := webutil.WaitForQuiescence(ctx, tab.conn, plTimeout); err != nil {
							return errors.Wrapf(err, "failed to wait for tab to achieve quiescence after scroll within %v", plTimeout)
						}
					}
				}
			}

			// Click on 1 link per 2 tabs, or click on 1 link for every tab under Record mode to ensure all links are
			// accessible under any other levels.
			if tabIdx%2 == 0 || caseLevel == Record {
				if err := tab.clickAnchor(ctx, plTimeout, tconn); err != nil {
					return errors.Wrap(err, "failed to click anchor")
				}
				if caseLevel == Record {
					// Ensure contents are renderred in recording mode.
					if err := webutil.WaitForRender(ctx, tab.conn, plTimeout); err != nil {
						return errors.Wrap(err, "failed to wait for render to finish")
					}
					if err := webutil.WaitForQuiescence(ctx, tab.conn, plTimeout); err != nil {
						return errors.Wrap(err, "failed to wait for tab to achieve quiescence")
					}
				} else {
					// It is normal that tabs might remain loading, hence no handle error here.
					webutil.WaitForQuiescence(ctx, tab.conn, clickLinkTimeout)
				}
				// Given some time after clicking any anchor before doing next operation.
				if err := testing.Sleep(ctx, time.Second); err != nil {
					return errors.Wrapf(err, "failed to sleep for %v", time.Second)
				}
			}
		}
	}
	return nil
}
