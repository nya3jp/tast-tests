// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type openLocation int

const (
	openInLacros              openLocation = iota // Opens in Lacros and not in Ash.
	openInLacrosAsUnreachable                     // Navigates to an unreachable page in Lacros.
	openInLacrosAsBlocked                         // Navigation gets blocked in Lacros.
	openInAsh                                     // Opens in Ash as application.
)

type urlRedirectParams struct {
	mode          openLocation
	url           string
	appTitleInAsh string
}

// This test is not testing all available URLs (go/lacros-url-redirect-links),
// but rather a random selection of URL's.

func init() {
	testing.AddTest(&testing.Test{
		Func:         URLRedirect,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Tests system URL redirect usage",
		Contacts:     []string{"skuhne@chromium.org", "hidehiko@chromium.org", "lacros-team@google.com"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "lacros",
		Timeout:      60 * time.Minute,
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
			Name: "browser_settings",
			Val: urlRedirectParams{mode: openInLacros,
				url: "chrome://settings/"},
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
			Val: urlRedirectParams{mode: openInAsh,
				url:           "os://credits/",
				appTitleInAsh: "ChromeOS-URLs - Credits"},
		}, {
			Name: "system_chrome_os_settings",
			Val: urlRedirectParams{mode: openInAsh,
				url:           "chrome://os-settings/",
				appTitleInAsh: "Settings"},
		}, {
			Name: "system_components",
			Val: urlRedirectParams{mode: openInAsh,
				url:           "os://components/",
				appTitleInAsh: "ChromeOS-URLs"},
		}, {
			Name: "system_credits",
			Val: urlRedirectParams{mode: openInAsh,
				url:           "chrome://os-credits/",
				appTitleInAsh: "ChromeOS-URLs - Credits"},
		}, {
			Name: "system_flags",
			Val: urlRedirectParams{mode: openInAsh,
				url:           "os://flags/",
				appTitleInAsh: "ChromeOS-URLs"},
		}, {
			Name: "system_internals",
			Val: urlRedirectParams{mode: openInAsh,
				url:           "os://sys-internals/",
				appTitleInAsh: "ChromeOS-URLs - System Internals"},
		}, {
			Name: "system_settings",
			Val: urlRedirectParams{mode: openInAsh,
				url:           "os://settings/",
				appTitleInAsh: "Settings"},
		}, {
			Name: "system_version",
			Val: urlRedirectParams{mode: openInAsh,
				url:           "os://version/",
				appTitleInAsh: "ChromeOS-URLs - About Version"},
		}, { // os:// URL should open an app on Ash side
			Name: "crosh",
			Val: urlRedirectParams{mode: openInAsh,
				url:           "chrome-untrusted://crosh/",
				appTitleInAsh: "crosh"},
		},
		},
	})
}

// URLRedirect is a basic test for lacros's intenral url redirect handling.
// It will open Lacros and navigate to a URL which should get either redirected
// to Ash, navigated to in Lacros, fails to navigate to or get even blocked.
func URLRedirect(ctx context.Context, s *testing.State) {
	// Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	params := s.Param().(urlRedirectParams)

	// Test prerequisites: no open windows.
	tconnAsh := s.FixtValue().(lacrosfixt.FixtValue).TestAPIConn()
	win, err := ash.GetAllWindows(ctx, tconnAsh)

	if len(win) != 0 {
		s.Fatal("There shouldn't be any open Ash or Lacros windows")
	}

	// Create a Lacros window.
	lacrosBrowser, err := lacros.Launch(ctx, s.FixtValue().(lacrosfixt.FixtValue))
	if err != nil {
		s.Fatal("Failed to launch lacros-chrome: ", err)
	}
	defer lacrosBrowser.Close(ctx)

	tconnLacros, err := lacrosBrowser.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to the Test API: ", err)
	}

	// Get a handle to the input keyboard.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard handle: ", err)
	}
	defer kb.Close()

	if err := uiauto.StartRecordFromKB(ctx, tconnLacros, kb); err != nil {
		s.Log("Failed to start recording: ", err)
	}

	defer uiauto.StopRecordFromKBAndSaveOnError(cleanupCtx, tconnLacros, s.HasError, s.OutDir())

	s.Log("Navigating in Lacros to the test URL: ", params.url)
	if err := navigateSingleTabToURLInLacros(ctx, params.url, lacrosBrowser, tconnAsh, kb, s); err != nil {
		s.Fatal("Failed to open a lacros tab: ", err)
	}

	// We use a matcher to allow either the URL, or the URL with a termiating "/"
	// or a blocked navigation.
	matcher := func(t *chrome.Target) bool {
		s.Log("Found target URL being used: ", t.URL)
		return t.URL == params.url || t.URL == (params.url+"/") || t.URL == "about:blank#blocked"
	}

	var appWindow *ash.Window

	// Depending on expected outcome we will test different things.
	if params.mode == openInAsh {
		// 1. Test that the window opened in Ash as application.
		s.Log("Testing that it got opened in Ash and not in Lacros - ", params.url)

		// Wait for application to appear on the Ash side.
		if err := ash.WaitForCondition(ctx, tconnAsh, func(w *ash.Window) bool {
			appWindow = w
			return true
		},
			&testing.PollOptions{Timeout: time.Minute, Interval: time.Second}); err != nil {
			s.Fatal("Waiting for app failed with error: ", err)
		}

		// Check Lacros side: If there is a tab, it should not show the (valid) URL.
		// Make sure that the current URL from Lacros is not the URL we navigated to.
		targets, err := lacrosBrowser.FindTargets(ctx, matcher)
		if err != nil {
			s.Fatal("Error when finding / matching Lacros window")
		}
		if len(targets) != 0 && targets[0].URL != params.url && targets[0].Attached {
			s.Fatal("Lacros should not navigate to this url", params.url)
		}

		// Check Ash side: There should be one app window of the known type.
		windows, title := getNumberOfAshWindows(ctx, tconnAsh, s)
		if windows != 1 || title != params.appTitleInAsh {
			// Seen that the title was not updated at the time the window showed.
			testing.Sleep(ctx, time.Second/2)
			windows, title := getNumberOfAshWindows(ctx, tconnAsh, s)
			if windows != 1 {
				s.Fatal("There should be only exactly one Ash window. but there are ",
					windows)
			}
			if title != params.appTitleInAsh {
				s.Fatal("This is not the correct app type:", title,
					" as it should have been:", params.appTitleInAsh)
			}
		}
	} else if params.mode == openInLacrosAsUnreachable {
		// 2. The URL is unknown and was not reachable.
		s.Log("Testing if opened in Lacros but prodces error - ", params.url)
		// Wait for navigation to finish.
		conn, err := lacrosBrowser.NewConnForTarget(ctx, matcher)

		// Note: As the navigation was done via keyboard, we didn't get any failure
		// and have to figure out the outcome of the tab (window) title.
		if err == nil || conn != nil {
			windows, title := getNumberOfLacrosWindows(ctx, tconnAsh, s)
			if windows != 1 {
				s.Fatal("One Lacros window expected, ", windows, " windows found")
			}
			if !isUnreachableNavigation(title, params.url) {
				s.Fatal("The navigation for " + params.url +
					" should have failed (found however: " + title + ")")
			}
		}
	} else if params.mode == openInLacrosAsBlocked {
		// 3. The URL navigation was blocked as it was a security risk.
		s.Log("Testing if opened in Lacros but prodces error - ", params.url)
		// Wait for navigation to finish.
		conn, err := lacrosBrowser.NewConnForTarget(ctx, matcher)

		// Note: As the navigation was done via keyboard, we didn't get any failure
		// and have to figure out the outcome of the tab (window) title.
		if err == nil || conn != nil {
			windows, title := getNumberOfLacrosWindows(ctx, tconnAsh, s)
			if windows != 1 {
				s.Fatal("One Lacros window expected, ", windows, " windows found")
			}
			if !isBlockedNavigation(title) {
				s.Fatal("The navigation to " + params.url + " should have been blocked")
			}
		}
	} else {
		// 4. The URL was successfully opened in Lacros.
		s.Log("Testing if opened correctly in Lacros and not in Ash - ", params.url)
		// Wait for navigation to finish.
		conn, err := lacrosBrowser.NewConnForTarget(ctx, matcher)

		if err != nil {
			s.Fatal("Error when finding / matching Lacros window")
		}
		if conn == nil {
			s.Fatal("Lacros should navigate to this url: ", params.url)
		}

		// Verify proper navigation.
		targets, err := lacrosBrowser.FindTargets(ctx, matcher)
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

		windows, title := getNumberOfLacrosWindows(ctx, tconnAsh, s)
		if windows != 1 {
			s.Fatal("One Lacros window expected, ", windows, " windows found")
		}
		if isUnreachableNavigation(title, params.url) || isBlockedNavigation(title) {
			s.Fatal("Navigation should have worked and not failed or get blocked")
		}
	}

	if appWindow != nil {
		err := appWindow.CloseWindow(ctx, tconnAsh)
		if err != nil {
			s.Fatal("Cannot close app window")
		}
	}

	// Need to finish first all activities before the close of the Lacros window
	// gets executed. It appears that some background extensions (YouTube) are
	// holding up the browser shut down.
	testing.Sleep(ctx, time.Second/2)
}

// navigateSingleTabToURLInLacros assumes that there's a freshly launched
// instance of lacros-chrome, with a single tab open (about:blank), then
// navigates that tab to the given url by using keyboard input so that the
// omnibox navigation takes place.
// Note that due to the fact that we navigate via keyboard, navigation problems
// (like invalid navigation) will not be able to be reported.
func navigateSingleTabToURLInLacros(ctx context.Context, url string, l *lacros.Lacros, tconnLacros *chrome.TestConn, keyboard *input.KeyboardEventWriter, s *testing.State) error {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Open a new tab and navigate to url.
	conn, err := l.NewConnForTarget(ctxWithTimeout, chrome.MatchTargetURL("about:blank"))
	if err != nil {
		return errors.Wrap(err, "failed to find an about:blank tab")
	}
	defer conn.Close()

	// We cannot use "conn.Navigate(ctx, url)" here, as that does not use the
	// omnibox navigation which should be used to get re-routed. As such we have
	// to enter the navigation into the omnibox to navigate.
	ui := uiauto.New(tconnLacros)
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

// getNumberOfAshWindows counts all Ash windows (Name: "BrowserFrame) and get
// the title of the first window to see the title of the app window.
func getNumberOfAshWindows(ctx context.Context, tconnAsh *chrome.TestConn, s *testing.State) (int, string) {
	win, err := ash.GetAllWindows(ctx, tconnAsh)
	if err != nil {
		s.Fatal("Cannot retrieve the open Ash windows: ", err)
	}
	title := ""
	ashWindows := 0
	for i := 0; i < len(win); i++ {
		if win[i].Name == "BrowserFrame" {
			title = win[i].Title
			ashWindows++
		}
	}

	return ashWindows, title
}

// getNumberOfLacrosWindows counts all Lacros windows (Name:"ExoShellSurface*")
// and get the Title of the first window to see what the navigation was doing.
func getNumberOfLacrosWindows(ctx context.Context, tconnLacros *chrome.TestConn, s *testing.State) (int, string) {
	win, err := ash.GetAllWindows(ctx, tconnLacros)
	if err != nil {
		s.Fatal("Cannot retrieve the open Lacros windows: ", err)
	}
	title := ""
	lacrosWindows := 0
	for i := 0; i < len(win); i++ {
		if strings.HasPrefix(win[i].Name, "ExoShellSurface") {
			title = win[i].Title
			lacrosWindows++
		}
	}

	return lacrosWindows, title
}

func isUnreachableNavigation(title, url string) bool {
	// In case of an unssuccesfull navigation the title will start with the URL
	// and may end with something like " - Google Chrome".
	return strings.HasPrefix(title, url)
}

func isBlockedNavigation(title string) bool {
	// In case of a blocked navigation the title will be fixed to the blocked URL
	// and end with something like " - Google Chrome".
	return strings.HasPrefix(title, "about:blank#blocked")
}
