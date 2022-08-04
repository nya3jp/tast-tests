// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

// This test is testing the usage of URL re-direction between Lacros and Ash
// to test that a URL (as can seen in go/lacros-url-redirect-links), is executed
// to the proper browser (Ash or Lacros) and then executed either as web page,
// as App or causes an error.
// Note that not all URL's are being tested here but rather a selection of URLs
// testing different use cases and outcomes.

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type openLocation int

// This enum describes how a particular URL should get opened when navigated to
// from the test in Lacros. It could cause an error to happen, get opened in
// Lacros on in Ash as an application.
const (
	openInLacros              openLocation = iota // Opens URL in Lacros
	openInLacrosAsUnreachable                     // Navigates to an unreachable page in Lacros
	openInLacrosAsBlocked                         // Navigation gets blocked in Lacros
	openInAshAsApplication                        // Opens in Ash as an application
)

// This structure defines the URL to test and the expected result.
type urlRedirectParams struct {
	mode             openLocation // How the URL navigation should proceed
	url              string       // The URL we want to navigate
	appTitleInAsh    string       // The application title as shown in Ash application
	tabTitleInLacros string       // The title of the tab in Lacros
	appID            string       // If an app this should be the ID
}

// init is the test initialization method.
func init() {
	testing.AddTest(&testing.Test{
		Func:         URLRedirect,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests system URL redirect usage",
		Contacts:     []string{"skuhne@chromium.org", "hidehiko@chromium.org", "lacros-team@google.com"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "lacros",
		Timeout:      15 * time.Minute,
	})
}

// URLRedirect is the function which iterates through all URLRedirect tests.
func URLRedirect(ctx context.Context, s *testing.State) {

	for _, tc := range []struct {
		subtest string
		params  urlRedirectParams
	}{
		{
			// chrome:// URL opens in Lacros
			subtest: "browser_components",
			params: urlRedirectParams{mode: openInLacros,
				url:              "chrome://components/",
				tabTitleInLacros: "Components - Google Chrome"},
		}, {
			subtest: "browser_credits",
			params: urlRedirectParams{mode: openInLacros,
				url:              "chrome://credits/",
				tabTitleInLacros: "Credits - Google Chrome"},
		}, {
			subtest: "browser_flags",
			params: urlRedirectParams{mode: openInLacros,
				url:              "chrome://flags/",
				tabTitleInLacros: "Experiments - Google Chrome"},
		}, {
			subtest: "browser_version",
			params: urlRedirectParams{mode: openInLacros,
				url:              "chrome://version/",
				tabTitleInLacros: "About Version - Google Chrome"},
		}, {
			// chrome:// URL's not opening in Lacros or Ash
			subtest: "no_lacros_sys_internals",
			params: urlRedirectParams{mode: openInLacrosAsUnreachable,
				url: "chrome://sys-internals/"},
		}, {
			// URLs which get blocked in Lacros and Ash.
			subtest: "unknown_blocked_os_link",
			params: urlRedirectParams{mode: openInLacrosAsBlocked,
				url: "os://unknown/"},
		}, {
			// os:// URL opening in Ash
			subtest: "ash_credits",
			params: urlRedirectParams{mode: openInAshAsApplication,
				url:           "os://credits/",
				appID:         "ohgadnbbmdopcjbkpfpmpafheioihjid",
				appTitleInAsh: "ChromeOS-URLs - Credits"},
		}, {
			subtest: "system_chrome_os_settings",
			params: urlRedirectParams{mode: openInAshAsApplication,
				url:           "chrome://os-settings/",
				appID:         "odknhmnlageboeamepcngndbggdpaobj",
				appTitleInAsh: "Settings"},
		}, {
			subtest: "system_components",
			params: urlRedirectParams{mode: openInAshAsApplication,
				url:           "os://components/",
				appID:         "ohgadnbbmdopcjbkpfpmpafheioihjid",
				appTitleInAsh: "ChromeOS-URLs - Components"},
		}, {
			subtest: "system_credits",
			params: urlRedirectParams{mode: openInAshAsApplication,
				url:           "chrome://os-credits/",
				appID:         "ohgadnbbmdopcjbkpfpmpafheioihjid",
				appTitleInAsh: "ChromeOS-URLs - Credits"},
		}, {
			subtest: "system_internals",
			params: urlRedirectParams{mode: openInAshAsApplication,
				url:           "os://sys-internals/",
				appID:         "ohgadnbbmdopcjbkpfpmpafheioihjid",
				appTitleInAsh: "ChromeOS-URLs - System Internals"},
		}, {
			subtest: "system_settings",
			params: urlRedirectParams{mode: openInAshAsApplication,
				url:           "os://settings/",
				appID:         "odknhmnlageboeamepcngndbggdpaobj",
				appTitleInAsh: "Settings"},
		}, {
			subtest: "system_version",
			params: urlRedirectParams{mode: openInAshAsApplication,
				url:           "os://version/",
				appID:         "ohgadnbbmdopcjbkpfpmpafheioihjid",
				appTitleInAsh: "ChromeOS-URLs - About Version"},
		}, { // os:// URL should open an app on Ash side
			subtest: "crosh",
			params: urlRedirectParams{mode: openInAshAsApplication,
				url:           "chrome-untrusted://crosh/",
				appID:         "cgfnfgkafmcdkdgilmojlnaadileaach",
				appTitleInAsh: "crosh"},
		},
	} {
		s.Run(ctx, tc.subtest, func(ctx context.Context, s *testing.State) {
			testURLRedirect(ctx, s, tc.params)
		})
	}
}

// testURLRedirect is a basic test for lacros's internal url redirect handling.
// It will open Lacros and navigate to a URL which should get either redirected
// to Ash, navigated to in Lacros, fail to navigate to or get even blocked.
func testURLRedirect(ctx context.Context, s *testing.State, params urlRedirectParams) {
	// Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Test prerequisites: no open windows.
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	atconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to the test API connection: ", err)
	}
	wins, err := ash.GetAllWindows(ctx, atconn)

	if err != nil {
		s.Fatal("Failed to get all windows: ", err)
	}
	if len(wins) != 0 {
		s.Log("there shouldn't be any open Ash or Lacros windows")
		// Let's close all windows.
		for _, w := range wins {
			w.CloseWindow(ctx, atconn)
		}
	}

	// Create a Lacros window.
	l, err := lacros.Launch(ctx, atconn)
	if err != nil {
		s.Fatal("Failed to launch lacros-chrome: ", err)
	}
	defer l.Close(cleanupCtx)

	// Get a handle to the input keyboard.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard handle: ", err)
	}
	defer kb.Close()
	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, params.url)

	s.Log("navigating in Lacros to the test URL: ", params.url)
	if err := navigateSingleTabToURLInLacros(ctx, params.url, l, atconn, kb); err != nil {
		s.Fatal("Failed to open a lacros tab: ", err)
	}

	matcher := chrome.MatchTargetURL(params.url)

	var appWindow *ash.Window

	// Depending on expected outcome we will test different things.
	switch params.mode {
	case openInAshAsApplication:
		// 1. Test that the window opened in Ash as application.
		s.Log("testing that it got opened in Ash and not in Lacros - ", params.url)

		// Wait for application to appear on the Ash side.
		if err := ash.WaitForCondition(ctx, atconn, func(w *ash.Window) bool {
			appWindow = w
			return w.AppID == params.appID && w.IsVisible
		}, &testing.PollOptions{Timeout: 20 * time.Second}); err != nil {
			s.Fatalf("Failed to wait for app to be visible for ID: %v with error %v", params.appID, err)
		}

		// Check Lacros side: If there is a tab, it should not show the (valid) URL.
		// Make sure that the current URL from Lacros is not the URL we navigated to.
		targets, err := l.FindTargets(ctx, matcher)
		if err != nil {
			s.Fatal("Error when finding / matching Lacros window: ", err)
		}
		if len(targets) != 0 && targets[0].URL != params.url && targets[0].Attached {
			s.Fatal("Lacros should not navigate to this url: ", params.url)
		}
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			wins, title := determineNumberOfAshWindowsAndTitle(ctx, atconn)
			if wins == 1 && title == params.appTitleInAsh {
				return nil
			}
			return errors.New("proper window not found")
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			s.Fatalf("Cannot find correct Ash application window (%v): %v", params.appTitleInAsh, err)
		}
	case openInLacrosAsUnreachable:
		// 2. The URL is unknown and was not reachable.
		s.Log("testing if opened in Lacros but produces error - ", params.url)
		// Wait for navigation to finish.
		conn, err := l.NewConnForTarget(ctx, matcher)
		if err != nil {
			s.Fatal("Cannot find target: ", err)
		}
		conn.Close()

		// Note: As the navigation was done via keyboard, we didn't get any failure
		// and have to figure out the outcome of the tab (window) title instead.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			wins, title := determineNumberOfLacrosWindowsAndTitle(ctx, atconn)
			if wins != 1 {
				return testing.PollBreak(errors.Errorf("number of lacros window mismatched: got %d, want 1", wins))
			}
			if unreachableNavigation(title, params.url) {
				return nil
			}
			return errors.New("proper window not found")
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			s.Fatal("Navigating to unreachable destination failed with: ", err)
		}
	case openInLacrosAsBlocked:
		// 3. The URL navigation was blocked as it was a security risk.
		s.Log("testing if opened in Lacros but produces error - ", params.url)
		// Wait for navigation to finish.
		matcher = func(t *chrome.Target) bool {
			s.Log("Found this page while looking for blocked page access: ", t.URL)
			return t.URL == "about:blank#blocked"
		}
		conn, err := l.NewConnForTarget(ctx, matcher)
		if err != nil {
			s.Fatal("Cannot find target: ", err)
		}
		conn.Close()
		// Note: As the navigation was done via keyboard, we didn't get any failure
		// and have to figure out the outcome of the tab (window) title.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			wins, title := determineNumberOfLacrosWindowsAndTitle(ctx, atconn)
			if wins != 1 {
				return testing.PollBreak(errors.Errorf("number of lacros window mismatched: got %d, want 1", wins))
			}
			if blockedNavigation(title) {
				return nil
			}
			return errors.New("proper window not found")
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			s.Fatal("Blocked navigation failed with: ", err)
		}
	case openInLacros:
		// 4. The URL was successfully opened in Lacros.
		s.Log("testing if opened correctly in Lacros and not in Ash - ", params.url)
		// Wait for navigation to finish.
		conn, err := l.NewConnForTarget(ctx, matcher)
		if err != nil {
			s.Fatal("Error when finding / matching Lacros window: ", err)
		}

		defer conn.Close()

		// Verify proper navigation.
		targets, err := l.FindTargets(ctx, matcher)
		if err != nil {
			s.Fatal("Error when finding / matching Lacros window: ", err)
		}
		if len(targets) == 0 {
			s.Fatal("There should have been at least one suitable target")
		}

		if !targets[0].Attached {
			s.Fatal("Navigation failed to: ", params.url)
		}
		if targets[0].URL != params.url {
			s.Fatal("Incorrect navigation to ", params.url)
		}

		// To make sure that nothing else went wrong (blocked or unreachable
		// navigation), we also check that the title has the proper name.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			wins, title := determineNumberOfLacrosWindowsAndTitle(ctx, atconn)
			if wins != 1 {
				return testing.PollBreak(errors.Errorf("number of lacros window mismatched: got %d, want 1", wins))
			}
			if title == params.tabTitleInLacros {
				return nil
			}
			return errors.New("proper window not found")
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			s.Fatal("Navigation did not lead to proper title: ", err)
		}
	}

	if appWindow != nil {
		err := appWindow.CloseWindow(ctx, atconn)
		if err != nil {
			s.Fatal("Cannot close app window: ", err)
		}
	}

	// Need to finish first all activities before the close of the Lacros window
	// gets executed. It appears that some background extensions (YouTube) are
	// holding up the browser shut down.
}

// navigateSingleTabToURLInLacros assumes that there's a freshly launched
// instance of lacros-chrome, with a single tab open (chrome://newtab/), then
// navigates that tab to the given url by using keyboard input so that the
// omnibox navigation takes place.
// Note that due to the fact that we navigate via keyboard, navigation problems
// (like invalid navigation) will not be able to be reported.
func navigateSingleTabToURLInLacros(ctx context.Context, url string, l *lacros.Lacros, tconn *chrome.TestConn, keyboard *input.KeyboardEventWriter) error {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Connect to a new tab and navigate to the url.
	conn, err := l.NewConnForTarget(ctxWithTimeout, chrome.MatchTargetURL(chrome.NewTabURL))
	if err != nil {
		return errors.Wrap(err, "failed to find a new tab page")
	}
	defer conn.Close()

	// We cannot use "conn.Navigate(ctx, url)" here, as that does not use the
	// omnibox navigation which should be used to get re-routed. As such we have
	// to enter the navigation into the omnibox to navigate.
	ui := uiauto.New(tconn)
	omniboxFinder := nodewith.Name("Address and search bar").Role(role.TextField)
	if err := uiauto.Combine("open target "+url,
		ui.LeftClick(omniboxFinder),
		keyboard.AccelAction("ctrl+a"),
		keyboard.TypeAction(url),
		keyboard.AccelAction("Enter"))(ctxWithTimeout); err != nil {
		return err
	}
	return nil
}

// determineNumberOfAshWindowsAndTitle counts all Ash windows
// (Name: "BrowserFrame) and get the title of the first window to see the title
// of the app window.
func determineNumberOfAshWindowsAndTitle(ctx context.Context, tconn *chrome.TestConn) (int, string) {
	ws, err := ash.FindAllWindows(ctx, tconn, func(w *ash.Window) bool {
		return w.Name == "BrowserFrame"
	})

	if err != nil || len(ws) == 0 {
		return 0, ""
	}
	return len(ws), ws[0].Title
}

// determineNumberOfLacrosWindowsAndTitle counts all Lacros windows
// (Name:"ExoShellSurface*") and get the Title of the first window to see what
// the navigation was doing.
func determineNumberOfLacrosWindowsAndTitle(ctx context.Context, tconn *chrome.TestConn) (int, string) {
	ws, err := ash.FindAllWindows(ctx, tconn, func(w *ash.Window) bool {
		return strings.HasPrefix(w.Name, "ExoShellSurface")
	})

	if err != nil || len(ws) == 0 {
		return 0, ""
	}
	return len(ws), ws[0].Title
}

func unreachableNavigation(title, url string) bool {
	// In case of an unsuccessful navigation the title will start with the URL
	// and may end with something like " - Google Chrome".
	return strings.HasPrefix(title, url)
}

func blockedNavigation(title string) bool {
	// In case of a blocked navigation the title will be fixed to the blocked URL
	// and end with something like " - Google Chrome".
	return strings.HasPrefix(title, "about:blank#blocked")
}
