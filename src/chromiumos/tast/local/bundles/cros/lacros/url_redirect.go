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
	mode          openLocation // How the URL navigation should proceed
	url           string       // The URL we want to navigate
	appTitleInAsh string       // The application title as shown in Ash application
	appID         string       // If an app this should be the ID
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
		Params: []testing.Param{{
			// chrome:// URL opens in Lacros
			Name: "browser_components",
			Val: urlRedirectParams{mode: openInLacros,
				url: "chrome://components/"},
		}, {
			Name: "browser_credits",
			Val: urlRedirectParams{mode: openInLacros,
				url: "chrome://credits/"},
		}, {
			Name: "browser_flags",
			Val: urlRedirectParams{mode: openInLacros,
				url: "chrome://flags/"},
		}, {
			Name: "browser_version",
			Val: urlRedirectParams{mode: openInLacros,
				url: "chrome://version/"},
		}, {
			// chrome:// URL's not opening in Lacros or Ash
			Name: "no_lacros_sys_internals",
			Val: urlRedirectParams{mode: openInLacrosAsUnreachable,
				url: "chrome://sys-internals/"},
		}, {
			// URLs which get blocked in Lacros and Ash.
			Name: "unknown_blocked_os_link",
			Val: urlRedirectParams{mode: openInLacrosAsBlocked,
				url: "os://unknown/"},
		}, {
			// os:// URL opening in Ash
			Name: "ash_credits",
			Val: urlRedirectParams{mode: openInAshAsApplication,
				url:           "os://credits/",
				appID:         "ohgadnbbmdopcjbkpfpmpafheioihjid",
				appTitleInAsh: "ChromeOS-URLs - Credits"},
		}, {
			Name: "system_chrome_os_settings",
			Val: urlRedirectParams{mode: openInAshAsApplication,
				url:           "chrome://os-settings/",
				appID:         "odknhmnlageboeamepcngndbggdpaobj",
				appTitleInAsh: "Settings"},
		}, {
			Name: "system_components",
			Val: urlRedirectParams{mode: openInAshAsApplication,
				url:           "os://components/",
				appID:         "ohgadnbbmdopcjbkpfpmpafheioihjid",
				appTitleInAsh: "ChromeOS-URLs - Components"},
		}, {
			Name: "system_credits",
			Val: urlRedirectParams{mode: openInAshAsApplication,
				url:           "chrome://os-credits/",
				appID:         "ohgadnbbmdopcjbkpfpmpafheioihjid",
				appTitleInAsh: "ChromeOS-URLs - Credits"},
		}, {
			Name: "system_internals",
			Val: urlRedirectParams{mode: openInAshAsApplication,
				url:           "os://sys-internals/",
				appID:         "ohgadnbbmdopcjbkpfpmpafheioihjid",
				appTitleInAsh: "ChromeOS-URLs - System Internals"},
		}, {
			Name: "system_settings",
			Val: urlRedirectParams{mode: openInAshAsApplication,
				url:           "os://settings/",
				appID:         "odknhmnlageboeamepcngndbggdpaobj",
				appTitleInAsh: "Settings"},
		}, {
			Name: "system_version",
			Val: urlRedirectParams{mode: openInAshAsApplication,
				url:           "os://version/",
				appID:         "ohgadnbbmdopcjbkpfpmpafheioihjid",
				appTitleInAsh: "ChromeOS-URLs - About Version"},
		}, { // os:// URL should open an app on Ash side
			Name: "crosh",
			Val: urlRedirectParams{mode: openInAshAsApplication,
				url:           "chrome-untrusted://crosh/",
				appID:         "cgfnfgkafmcdkdgilmojlnaadileaach",
				appTitleInAsh: "crosh"},
		},
		},
	})
}

// URLRedirect is a basic test for lacros's internal url redirect handling.
// It will open Lacros and navigate to a URL which should get either redirected
// to Ash, navigated to in Lacros, fail to navigate to or get even blocked.
func URLRedirect(ctx context.Context, s *testing.State) {
	// Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	params := s.Param().(urlRedirectParams)

	// Test prerequisites: no open windows.
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	atconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to the test API connection: ", err)
	}
	wins, err := ash.GetAllWindows(ctx, atconn)

	if len(wins) != 0 {
		s.Log("There shouldn't be any open Ash or Lacros windows")
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

	s.Log("Navigating in Lacros to the test URL: ", params.url)
	if err := navigateSingleTabToURLInLacros(ctx, params.url, l, atconn, kb); err != nil {
		s.Fatal("Failed to open a lacros tab: ", err)
	}

	// We use a matcher to allow either the URL or a blocked navigation.
	matcher := func(t *chrome.Target) bool {
		s.Log("Found target URL being used: ", t.URL)
		return t.URL == params.url
	}

	var appWindow *ash.Window

	// Depending on expected outcome we will test different things.
	if params.mode == openInAshAsApplication {
		// 1. Test that the window opened in Ash as application.
		s.Log("Testing that it got opened in Ash and not in Lacros - ", params.url)

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
			s.Fatal("Error when finding / matching Lacros window")
		}
		if len(targets) != 0 && targets[0].URL != params.url && targets[0].Attached {
			s.Fatal("Lacros should not navigate to this url", params.url)
		}

		// Check Ash side: There should be one app window of the known type.
		wins, title := determineNumberOfAshWindowsAndTitle(ctx, atconn)
		if wins != 1 || title != params.appTitleInAsh {
			// Seen that the title was not updated at the time the window showed.
			testing.Sleep(ctx, time.Second/2)
			wins, title := determineNumberOfAshWindowsAndTitle(ctx, atconn)
			if wins != 1 {
				s.Fatal("There should be only exactly one Ash window. but there are ",
					wins)
			}
			if title != params.appTitleInAsh {
				s.Fatal("This is not the correct app type:", title,
					" as it should have been:", params.appTitleInAsh)
			}
		}
	} else if params.mode == openInLacrosAsUnreachable {
		// 2. The URL is unknown and was not reachable.
		s.Log("Testing if opened in Lacros but produces error - ", params.url)
		// Wait for navigation to finish.
		conn, err := l.NewConnForTarget(ctx, matcher)
		// Note: As the navigation was done via keyboard, we didn't get any failure
		// and have to figure out the outcome of the tab (window) title.
		if err == nil || conn != nil {
			wins, title := determineNumberOfLacrosWindowsAndTitle(ctx, atconn)
			if wins != 1 {
				s.Fatal("One Lacros window expected, ", wins, " windows found")
			}
			if !isUnreachableNavigation(title, params.url) {
				s.Fatal("The navigation for " + params.url +
					" should have failed (found however: " + title + ")")
			}
		}
	} else if params.mode == openInLacrosAsBlocked {
		// 3. The URL navigation was blocked as it was a security risk.
		s.Log("Testing if opened in Lacros but produces error - ", params.url)
		// Wait for navigation to finish.
		matcher = func(t *chrome.Target) bool {
			s.Log("Found blocked page access: ", t.URL)
			return t.URL == "about:blank#blocked"
		}
		conn, err := l.NewConnForTarget(ctx, matcher)

		// Note: As the navigation was done via keyboard, we didn't get any failure
		// and have to figure out the outcome of the tab (window) title.
		if err == nil || conn != nil {
			wins, title := determineNumberOfLacrosWindowsAndTitle(ctx, atconn)
			if wins != 1 {
				s.Fatal("One Lacros window expected, ", wins, " windows found")
			}
			if !isBlockedNavigation(title) {
				s.Fatal("The navigation to " + params.url + " should have been blocked")
			}
		}
	} else {
		// 4. The URL was successfully opened in Lacros.
		s.Log("Testing if opened correctly in Lacros and not in Ash - ", params.url)
		// Wait for navigation to finish.
		conn, err := l.NewConnForTarget(ctx, matcher)

		if err != nil {
			s.Fatal("Error when finding / matching Lacros window")
		}
		if conn == nil {
			s.Fatal("Lacros should navigate to this url: ", params.url)
		}

		// Verify proper navigation.
		targets, err := l.FindTargets(ctx, matcher)
		if len(targets) == 0 {
			s.Fatal("Error there should have been at least one suitable target")
		}

		if err != nil {
			s.Fatal("Error when findingmatching Lacros window")
		}

		if !targets[0].Attached {
			s.Fatal("Navigation failed to: ", params.url)
		}
		if targets[0].URL != params.url {
			s.Fatal("Incorrect navigation to ", params.url)
		}

		wins, title := determineNumberOfLacrosWindowsAndTitle(ctx, atconn)
		if wins != 1 {
			s.Fatal("One Lacros window expected, ", wins, " windows found")
		}
		if isUnreachableNavigation(title, params.url) || isBlockedNavigation(title) {
			s.Fatal("Navigation should have worked and not failed or get blocked")
		}
	}

	if appWindow != nil {
		err := appWindow.CloseWindow(ctx, atconn)
		if err != nil {
			s.Fatal("Cannot close app window")
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
		return errors.Wrap(err, "failed to find an new tab page")
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

	// As the navigation has been done via keys, there is no good way to see if
	// the navigation is 100% finished. E.g.: Not giving the two browsers time,
	// can lead to a non updated title. Without a tiny sleep here, the tests
	// "system_comnponents" and "no_lacros_sys_internals" become flaky.
	testing.Sleep(ctx, time.Second/2)
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

func isUnreachableNavigation(title, url string) bool {
	// In case of an unsuccessful navigation the title will start with the URL
	// and may end with something like " - Google Chrome".
	return strings.HasPrefix(title, url)
}

func isBlockedNavigation(title string) bool {
	// In case of a blocked navigation the title will be fixed to the blocked URL
	// and end with something like " - Google Chrome".
	return strings.HasPrefix(title, "about:blank#blocked")
}
