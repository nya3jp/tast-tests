// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"fmt"

	// "io/ioutil"
	// "regexp"
	// "strconv"
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
	openInLacros openLocation = iota // Opens in Lacros and not in Ash.
	openInLacrosAs404                // Navigates to a 404 in Lacros.
	openInAsh                        // Opens in Ash as application.
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
		Func:         UrlRedirect,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Tests system URL redirect usage",
		Contacts:     []string{"skuhne@chromium.org", "hidehiko@chromium.org", "lacros-team@google.com"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "lacros",
		Timeout:      60 * time.Minute,
		Params: []testing.Param{{
/* TODO: Uncomment
			// chrome:// URL opens in Lacros
				Name: "browser_components",
				Val:  urlRedirectParams{mode: openInLacros,
																url: "chrome://components/"},
			}, {
				Name: "browser_credits",
				Val:  urlRedirectParams{mode: openInLacros,
																url: "chrome://credits/"},
			}, {
				Name: "browser_flags",
				Val:  urlRedirectParams{mode: openInLacros,
																url: "chrome://flags/"},
			}, {
				Name: "browser_version",
				Val:  urlRedirectParams{mode: openInLacros,
																url: "chrome://version/"},
			}, {
*/
				Name: "browser_settings",
				Val:  urlRedirectParams{mode: openInLacros,
																url: "chrome://settings/"},
			}, {
				// chrome:// URL's not opening in Lacros or Ash
				Name: "no_lacros_sys_internals",
				Val:  urlRedirectParams{mode: openInLacrosAs404,
																url: "chrome://sys-internals/"},
/* TODO: Uncomment
			}, {
				// os:// URL opening in Ash
				Name: "ash_credits",
				Val:  urlRedirectParams{mode: openInAsh,
																url: "os://credits/",
																appTitleInAsh: "ChromeOS-URLs - Credits"},
			}, {
				Name: "system_chrome_os_settings",
				Val:  urlRedirectParams{mode: openInAsh,
																url: "chrome://os-settings/",
																appTitleInAsh: "Settings"},
			}, {
				Name: "system_components",
				Val:  urlRedirectParams{mode: openInAsh,
																url: "os://components/",
																appTitleInAsh: "ChromeOS-URLs"},
			}, {
				Name: "system_credits",
				Val:  urlRedirectParams{mode: openInAsh,
																url: "chrome://os-credits/",
																appTitleInAsh: "ChromeOS-URLs - Credits"},
			}, {
				Name: "system_flags",
				Val:  urlRedirectParams{mode: openInAsh,
																url: "os://flags/",
																appTitleInAsh: "ChromeOS-URLs"},
			}, {
				Name: "system_internals",
				Val:  urlRedirectParams{mode: openInAsh,
																url: "os://sys-internals/",
																appTitleInAsh: "ChromeOS-URLs - System Internals"},
			}, {
				Name: "system_settings",
				Val:  urlRedirectParams{mode: openInAsh,
																url: "os://settings/",
																appTitleInAsh: "Settings"},
			}, {
				Name: "system_version",
				Val:  urlRedirectParams{mode: openInAsh,
																url: "os://version/",
																appTitleInAsh: "ChromeOS-URLs - About Version"},
			}, { // os:// URL should open an app on Ash side
				Name: "crosh",
				Val:  urlRedirectParams{mode: openInAsh,
																url: "chrome-untrusted://crosh/",
																appTitleInAsh: "crosh"},
*/
			},
	  },
	})
}

// UrlRedirect is a basic test for lacros's intenral url redirect handling.
// It will open Lacros and navigate to a URL which should get redirected to
// Ash. Within Ash it will click then on the header URL and check that that will
// redirect back to Lacros.
func UrlRedirect(ctx context.Context, s *testing.State) {
	// Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

  params := s.Param().(urlRedirectParams)

	// Test prequisites: no open windows.
	tconnAsh := s.FixtValue().(lacrosfixt.FixtValue).TestAPIConn()
	windows, _ := GetNumberOfAshWindows(ctx, tconnAsh, s)
  if windows != 0 {
		s.Fatal("There shouldn't be any open Ash windows!")
	}

  // Create a Lacros window.
	lacros_browser, err := lacros.Launch(ctx, s.FixtValue().(lacrosfixt.FixtValue))
	if err != nil {
		s.Fatal("Failed to launch lacros-chrome: ", err)
	}

	tconnLacros,err := lacros_browser.TestAPIConn(ctx)
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
	if err := navigateSingleTabToURLInLacros(ctx, params.url, lacros_browser, tconnAsh, kb, s); err != nil {
		s.Fatal("Failed to open a lacros tab: ", err)
	}

	matcher := func(t *chrome.Target) bool {
		s.Log("Found target URL being used: ", t.URL)
		return t.URL == params.url || t.URL == (params.url + "/")
	}

  var appWindow *ash.Window

  if params.mode == openInAsh {
    s.Log("Testing that it got opened in Ash and not in Lacros - ", params.url)
		// ash_browser := s.FixtValue().(lacrosfixt.FixtValue).Chrome()
		// Wait for application to appear on the Ash side.
    if err := ash.WaitForCondition(ctx, tconnAsh, func (w *ash.Window) bool {
			  appWindow = w
				return true},
		  &testing.PollOptions{Timeout: time.Minute, Interval: time.Second}); err != nil {
				s.Fatal("Waiting for app failed with error: ", err)
		}

	  // Check Lacros side: If there is a tab, it should not show the (valid) URL.

		// Make sure that the current URL from Lacros is not the URL we navigated to.
		targets, err := lacros_browser.FindTargets(ctx, matcher)
		if err != nil {
			s.Fatal("Error when findingmatching Lacros window!")
		}
		if len(targets) != 0 && targets[0].URL != params.url && targets[0].Attached {
			s.Fatal("Lacros should not navigate to this url!", params.url)
		}

		// Check Ash side: There should be an App of the known type.

		// Make sure we have only one window.
		windows, title := GetNumberOfAshWindows(ctx, tconnAsh, s)
    if windows != 1 || title != params.appTitleInAsh {
			// Seen that the title was not updated at the time the window showed.
			testing.Sleep(ctx, time.Second / 2)
			windows, title := GetNumberOfAshWindows(ctx, tconnAsh, s)
			if windows != 1 {
				s.Fatal("There should be only exactly one Ash window! (is:", windows, ")")
			}
			if title != params.appTitleInAsh {
				s.Fatal("This is not the correct app type! title=", title, " should be=", params.appTitleInAsh)
  		}
		}
		// The passed AppID from the framework is not populated. Otherwise we could
		// test for that.
    // if appWindow.FullRestoreWindowAppID != params.appIdInAsh {
		//	s.Fatal("This is not the correct app!")
		//}
	} else if params.mode == openInLacrosAs404 {
    s.Log("Testing if opened in Lacros but prodces error - ", params.url)
		// Wait for navigation to finish.
		// Make sure that the current URL from Lacros is not the URL we navigated to.
		conn, err := lacros_browser.NewConnForTarget(ctx, matcher)

		if err == nil || conn != nil {
			s.Fatal("The navigation for " + params.url + " should have failed!")
		}

	} else {
    s.Log("Testing if opened correctly in Lacros and not in Ash - ", params.url)
		// Wait for navigation to finish.
		// Make sure that the current URL from Lacros is not the URL we navigated to.
		conn, err := lacros_browser.NewConnForTarget(ctx, matcher)

		if err != nil {
			s.Fatal("Error when findingmatching Lacros window!")
		}
		if conn == nil {
			s.Fatal("Lacros should not navigate to this url!", params.url)
		}

		// Verify proper navigation.
		targets, err := lacros_browser.FindTargets(ctx, matcher)
		if len(targets) == 0 {
			s.Fatal("Error there should have been at least one target created!")
		}

		if err != nil {
			s.Fatal("Error when findingmatching Lacros window!")
		}

		s.Log("InLacros:all= ", fmt.Sprintf("%+v\n", targets[0]))

		if !targets[0].Attached {
		  s.Fatal("Navigation failed to: ", params.url)
		}
		if targets[0].URL != params.url {
		  s.Fatal("Incorrect navigation to ", params.url)
		}
	}

  if appWindow != nil {
		err := appWindow.CloseWindow(ctx, tconnAsh)
		if err != nil {
			s.Fatal("Cannot close app window!")
		}
	}

	// Close lacros-chrome
	lacros_browser.Close(ctx)
}

// navigateSingleTabToURLInLacros assumes that there's a freshly launched instance
// of lacros-chrome, with a single tab open to about:blank, then, navigates the
// blank tab to the given url.
func navigateSingleTabToURLInLacros(ctx context.Context, url string, l *lacros.Lacros, tconnLacros *chrome.TestConn, keyboard *input.KeyboardEventWriter, s *testing.State) error {
  ctxWithTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
  defer cancel()

	// Open a new tab and navigate to url.
	conn, err := l.NewConnForTarget(ctx, chrome.MatchTargetURL("about:blank"))
	if err != nil {
		return errors.Wrap(err, "failed to find an about:blank tab")
	}
	defer conn.Close()

	// We cannot use "conn.Navigate(ctx, url)" here, as that does not use the
	// omnibox navigation which should be used to get re-routed. As such we have
	// to enter the navigation into the omnibox to navigate.
	ui := uiauto.New(tconnLacros)
	omniboxFinder := nodewith.Name("Address and search bar").Role(role.TextField)
	if err := uiauto.Combine("open target "+ url,
		ui.LeftClick(omniboxFinder),
		keyboard.AccelAction("ctrl+a"),
		keyboard.TypeAction(url),
		keyboard.AccelAction("Enter"))(ctxWithTimeout); err != nil {
			return err
	}

	// As the navigation was done indirectly, we didn't get any navigation failure
	// and have to figure that now out separately.

  // TODO: Determine if the navigation was a success or if it failed.

	return nil
}

func GetNumberOfAshWindows(ctx context.Context, tconnAsh *chrome.TestConn, s *testing.State) (int, string) {
	win, err := ash.GetAllWindows(ctx, tconnAsh)
	if err != nil {
		s.Fatal("Cannot retrieve the open Ash windows: ", err)
	}
	// we need to count all windows which are not of Name = "BrowserFrame"
	// ignoring Name:ExoShellSurface-3 and look at the Title which should be
	// Title:ChromeOS-URLs and not Title:about-blank
	title := ""
	ashWindows := 0
  for i := 0; i < len(win); i+=1 {
		if win[i].Name == "BrowserFrame" {
			title = win[i].Title
			ashWindows++
		}
	}

	return ashWindows, title
}
