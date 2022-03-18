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

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/testing"
)

type openLocation int

const (
	openInLacros openLocation = iota
	openInAsh
)

type urlRedirectParams struct {
  mode       openLocation
	url        string
	backLink   bool
	appIdInAsh string
}

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
			// Normal URL opens in Lacros (test works)
			Name: "browser_flags",
			Val:  urlRedirectParams{mode: openInLacros,
				                      url: "chrome://flags/",
														  backLink: true},
		}, {
	    // os:// URL opening in Ash (test waits for ever)
//			Name: "system_flags",
//			Val:  urlRedirectParams{mode: openInAsh,
//	                            url: "os://flags/",
//														  backLink: true},
//		}, {
	    // Normal URL opens in Lacros (test works)
			Name: "browser_version",
			Val:  urlRedirectParams{mode: openInLacros,
				                      url: "chrome://version/",
													 	  backLink: true},
		}, {
	    // os:// URL opening in Ash (test waits for ever)
//			Name: "system_version",
//			Val:  urlRedirectParams{mode: openInAsh,
//	                            url: "os://version/",
//															backLink: true},
//	  }, {
	    // os:// URL should open an app on Ash side (test passes)
			Name: "crosh",
			Val:  urlRedirectParams{mode: openInAsh,
				                      url: "chrome-untrusted://crosh/",
															backLink: false,
														  appIdInAsh: "behllobkkfkfnphdnhnkndlbkcpglgmj"},
		},
	  },
	})
}

// UrlRedirect is a basic test for lacros's intenral url redirect handling.
// It will open Lacros and navigate to a URL which should get redirected to
// Ash. Within Ash it will click then on the header URL and check that that will
// redirect back to Lacros.
func UrlRedirect(ctx context.Context, s *testing.State) {
  params := s.Param().(urlRedirectParams)

	// Test prequisites: no open windows.
	tconnAsh := s.FixtValue().(lacrosfixt.FixtValue).TestAPIConn()
  if GetNumberOfAshWindows(ctx, tconnAsh, s) != 0 {
		s.Fatal("There shouldn't be any open Ash windows!")
	}

  // Create a Lacros window.
	lacros_browser, err := lacros.Launch(ctx, s.FixtValue().(lacrosfixt.FixtValue))
	if err != nil {
		s.Fatal("Failed to launch lacros-chrome: ", err)
	}

  s.Log("Navigating in Lacros to the test URL: ", params.url)
	if err := navigateSingleTabToURLInLacros(ctx, params.url, lacros_browser); err != nil {
		s.Fatal("Failed to open a lacros tab: ", err)
	}

	matcher := func(t *chrome.Target) bool {
		s.Log("Found target URL being used: ", t.URL)
		return t.URL == params.url || t.URL == (params.url + "/")
	}

  var appWindow *ash.Window

  if params.mode == openInAsh {
    s.Log("Testing thati it got opened in Ash and not in Lacros - ", params.url)
		// ash_browser := s.FixtValue().(lacrosfixt.FixtValue).Chrome()

		// Wait for application to appear on the Ash side.
		// TODO: I think this does not work!!
    if err := ash.WaitForCondition(ctx, tconnAsh, func (w *ash.Window) bool {
			  appWindow = w
				return true},
		  &testing.PollOptions{Timeout: time.Minute, Interval: time.Second}); err != nil {
			s.Fatal("Waiting for app failed with error: ", err)
		}

		// TODO: Remove - only left here to see the result.
		testing.Sleep(ctx, 5 * time.Second)

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
		if GetNumberOfAshWindows(ctx, tconnAsh, s) != 1 {
			s.Fatal("There should be only exactly one Ash window!")
		}

		// Check the AppId is correct.
    if appWindow.FullRestoreWindowAppID != params.appIdInAsh {
			s.Fatal("This is not the correct app!")
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

/* If we want to check clicking the back link we should add this...
  if params.backLink {
		// Click the backink...
		if err := s.FixtValue().(lacrosfixt.FixtValue).TestAPIConn().Call(ctx, nil, `async () => {
			const win = await tast.promisify(chrome.windows.getLastFocused)();
			await tast.promisify(chrome.windows.update)(win.id, {width: 800, height:600, state:"normal"});
		}`); err != nil {
			s.Fatal("Backlink couldn't be clicked: ", err)
		}

		// Now do the same test as above - only the other direction.
		if params.mode == openInAsh {
      ..
		}

		// Last but not least we might want to check that only a single instance
		// will be opened.
	}
*/

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
func navigateSingleTabToURLInLacros(ctx context.Context, url string, l *lacros.Lacros) error {
	// Open a new tab and navigate to url.
	conn, err := l.NewConnForTarget(ctx, chrome.MatchTargetURL("about:blank"))
	if err != nil {
		return errors.Wrap(err, "failed to find an about:blank tab")
	}
	defer conn.Close()
	if err := conn.Navigate(ctx, url); err != nil {
		return errors.Wrapf(err, "failed to navigate to %q", url)
	}
	return nil
}

func GetNumberOfAshWindows(ctx context.Context, tconnAsh *chrome.TestConn, s *testing.State) int {
	win, err := ash.GetAllWindows(ctx, tconnAsh)
	if err != nil {
		s.Fatal("Cannot retrieve the open Ash windows: ", err)
	}
	return len(win)
}
