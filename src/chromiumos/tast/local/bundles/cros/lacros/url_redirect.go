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
	openInLacros openLocation = iota
	openInAsh
)

type urlRedirectParams struct {
  mode          openLocation
	url           string
	backLink      bool
	appIdInAsh    string
	appTitleInAsh string
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
			Name: "system_flags",
			Val:  urlRedirectParams{mode: openInAsh,
	                            url: "os://flags/",
															appTitleInAsh: "ChromeOS-URLs",
														  backLink: true},
		}, {
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
															appTitleInAsh: "crosh",
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
s.Log("Waiting for app...")
		// Wait for application to appear on the Ash side.
		// TODO: I think this does not work!!
    if err := ash.WaitForCondition(ctx, tconnAsh, func (w *ash.Window) bool {
			  appWindow = w
				return true},
		  &testing.PollOptions{Timeout: time.Minute, Interval: time.Second}); err != nil {
s.Log("oopsie ", err)
				s.Fatal("Waiting for app failed with error: ", err)
		}
s.Log("Waited and something has happened")

		// TODO: Remove - only left here to see the result.
		testing.Sleep(ctx, 5 * time.Second)

	  // Check Lacros side: If there is a tab, it should not show the (valid) URL.
s.Log("Waited additional 5s's")

		// Make sure that the current URL from Lacros is not the URL we navigated to.
		targets, err := lacros_browser.FindTargets(ctx, matcher)

		if err != nil {
s.Log("Error enumerating ", err)
			s.Fatal("Error when findingmatching Lacros window!")
		}
s.Log("Targets: ", len(targets))
if len(targets) > 0 {
s.Log("URL[0]: ", targets[0].URL)
s.Log("Attached[0] " , targets[0].Attached)
if len(targets) > 1 {
s.Log("URL[1]: ", targets[1].URL)
s.Log("Attached[1] " , targets[1].Attached)
}
}
		if len(targets) != 0 && targets[0].URL != params.url && targets[0].Attached {
			s.Fatal("Lacros should not navigate to this url!", params.url)
		}

		// Check Ash side: There should be an App of the known type.

		// Make sure we have only one window.
		windows, title := GetNumberOfAshWindows(ctx, tconnAsh, s)
s.Log("Number of Ash windows & title=(", windows, ", ", title, ")")
		if windows != 1 {
			s.Fatal("There should be only exactly one Ash window!")
		}
    if title != params.appTitleInAsh {
			s.Fatal("This is not the correct app type! title=", title, " should be=", params.appTitleInAsh)
		}
		// Check the AppId is correct.
		// Note: Currently not populated by framework.
    // if appWindow.FullRestoreWindowAppID != params.appIdInAsh {
		//	s.Fatal("This is not the correct app!")
		//}

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
func navigateSingleTabToURLInLacros(ctx context.Context, url string, l *lacros.Lacros, tconnLacros *chrome.TestConn, keyboard *input.KeyboardEventWriter, s *testing.State) error {
/*
	// Open a new tab and navigate to url.
	conn, err := l.NewConnForTarget(ctx, chrome.MatchTargetURL("about:blank"))
	if err != nil {
		return errors.Wrap(err, "failed to find an about:blank tab")
	}
	defer conn.Close()
	if err := conn.Navigate(ctx, url); err != nil {
		return errors.Wrapf(err, "failed to navigate to %q", url)
	}
*/

  ctxWithTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
  defer cancel()

	// Open a new tab and navigate to url.
	conn, err := l.NewConnForTarget(ctx, chrome.MatchTargetURL("about:blank"))
	if err != nil {
		return errors.Wrap(err, "failed to find an about:blank tab")
	}
	defer conn.Close()

	ui := uiauto.New(tconnLacros)
	omniboxFinder := nodewith.Name("Address and search bar").Role(role.TextField)
	if err := uiauto.Combine("open target "+ url,
		ui.LeftClick(omniboxFinder),
		keyboard.AccelAction("ctrl+a"),
		keyboard.TypeAction(url),
		keyboard.AccelAction("Enter"))(ctxWithTimeout); err != nil {
			return err
	}

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
s.Log("Win found with Title ", title)
			title = win[i].Title
			ashWindows++
		}
	}

if len(win) >= 3 {
s.Log("GetNumberOfAshWindows: ", fmt.Sprintf("\n1: %+v\n2: %+v\n3: %+v\n", win[0], win[1], win[2]))
}else if len(win) >= 2 {
s.Log("GetNumberOfAshWindows: ", fmt.Sprintf("\n1: %+v\n2: %+v\n", win[0], win[1]))
}else if len(win) >= 1 {
s.Log("GetNumberOfAshWindows: ", fmt.Sprintf("\n1: %+v\n", win[0]))
} else {
s.Log("GetNumberOfAshWindows: 0")
}
	return ashWindows, title
}
