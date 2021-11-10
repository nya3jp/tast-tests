// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package taskmanager

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/cws"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/taskmanager"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DisplayAllTabs,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Test that all tabs should be displayed in the task manager",
		Contacts: []string{
			"sun.tsai@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		// GAIA is required to install an app from Chrome Webstore.
		Fixture: "chromeLoggedWithGaia",
	})
}

// DisplayAllTabs tests that all tabs should be displayed in the task manager.
func DisplayAllTabs(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get the connection to the test API: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	ui := uiauto.New(tconn)

	// Expecting 3 windows, 2 tabs on the first window, 4 tabs on the second and the third window.
	browserTabs := [][]taskmanager.Process{
		{
			newChromeTabInNewWindow("https://www.facebook.com/"),
			newExtensionTab(chrome.BlankURL, "Speedtest"),
		}, {
			newChromeTabInNewWindow("https://www.amazon.com/"),
			taskmanager.NewChromeTabProcess("https://www.apple.com/"),
			newYoutubeTab("https://www.youtube.com/"),
			taskmanager.NewChromeTabProcess("https://www.instagram.com/"),
		}, {
			newChromeTabInNewWindow("https://en.wikipedia.org/wiki/Main_Page"),
			taskmanager.NewChromeTabProcess("https://news.google.com/"),
			taskmanager.NewChromeTabProcess("https://news.ycombinator.com/news"),
			taskmanager.NewChromeTabProcess("https://www.cbc.ca/lite/trending-news"),
		},
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cwsApp := cws.App{Name: cwsAppName, URL: cwsAppURL}
	if err := cws.InstallApp(ctx, cr, tconn, cwsApp); err != nil {
		s.Fatal("Failed to install CWS app: ", err)
	}
	defer cws.UninstallApp(cleanupCtx, cr, tconn, cwsApp)

	for _, browserWindow := range browserTabs {
		for _, process := range browserWindow {
			if err := process.Open(ctx, cr, tconn, kb); err != nil {
				s.Fatalf("Failed to open %q: %v", process.NameInTaskManager(), err)
			}
			defer process.Close(cleanupCtx)
		}
	}

	tm := taskmanager.New(tconn, kb)
	if err := tm.Open(ctx); err != nil {
		s.Fatal("Failed to launch the task manager: ", err)
	}
	defer tm.Close(cleanupCtx, tconn)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_dump")

	for _, browserWindow := range browserTabs {
		for _, process := range browserWindow {
			// The processes might be grouped. Such as the extension "Speedtest" in this test.
			if err := ui.WaitUntilExists(taskmanager.FindProcess().NameStartingWith(process.NameInTaskManager()).First())(ctx); err != nil {
				s.Fatalf("Failed to find process %q in task manager: %v", process.NameInTaskManager(), err)
			}
		}
	}
}

// Speedtest by Ookla is an extension which can be used to test internet performance.
const (
	cwsAppID   = "pgjjikdiikihdfpoppgaidccahalehjh"
	cwsAppURL  = "https://chrome.google.com/webstore/detail/speedtest-by-ookla/" + cwsAppID
	cwsAppName = "Speedtest by Ookla"
)

func newChromeTabInNewWindow(url string) *taskmanager.ChromeTab {
	tab := taskmanager.NewChromeTabProcess(url)
	tab.SetOpenInNewWindow()
	return tab
}

// extenstionTab represents the tab where the extension is opened.
type extensionTab struct {
	*taskmanager.ChromeTab
	name string
}

func newExtensionTab(url, name string) *extensionTab {
	return &extensionTab{
		ChromeTab: taskmanager.NewChromeTabProcess(url),
		name:      name,
	}
}

func newExtensionTabInNewWindow(url, name string) *extensionTab {
	tab := newExtensionTab(url, name)
	tab.SetOpenInNewWindow()
	return tab
}

// Open overrides the ChromeTab.Open() to open the installed extension.
func (tab *extensionTab) Open(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, kb *input.KeyboardEventWriter) error {
	if err := tab.ChromeTab.Open(ctx, cr, tconn, kb); err != nil {
		return err
	}

	ui := uiauto.New(tconn)
	browserFrame := nodewith.HasClass("BrowserFrame").Role(role.Window)
	extensionMenu := nodewith.HasClass("ExtensionsMenuView").Role(role.Window)

	return uiauto.Combine("open the extension",
		ui.LeftClick(nodewith.Name("Extensions").Role(role.PopUpButton).Ancestor(browserFrame)),
		ui.LeftClick(nodewith.NameStartingWith(tab.name).HasClass("ExtensionsMenuButton").Ancestor(extensionMenu)),
		ui.WaitUntilExists(nodewith.Name(tab.name).Role(role.RootWebArea)),
	)(ctx)
}

func (tab *extensionTab) NameInTaskManager() string {
	return "Extension: " + tab.name
}

type youtubeTab struct {
	*taskmanager.ChromeTab
	cwsAppInstalled bool
}

func newYoutubeTab(url string) *youtubeTab {
	return &youtubeTab{
		ChromeTab: taskmanager.NewChromeTabProcess(url),
	}
}

func newYoutubeTabInNewWindow(url string) *youtubeTab {
	tab := newYoutubeTab(url)
	tab.SetOpenInNewWindow()
	return tab
}

func (tab *youtubeTab) Open(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, kb *input.KeyboardEventWriter) error {
	if err := tab.ChromeTab.Open(ctx, cr, tconn, kb); err != nil {
		return err
	}

	// If the YouTube app from Chrome Web Store is installed, it will be used to open the YouTube link by default.
	// The displayed name in the task manager will be different.
	cwsAppInstalled, err := ash.ChromeAppInstalled(ctx, tconn, apps.YouTubeCWS.ID)
	if err != nil {
		return errors.Wrap(err, "failed to get Chrome Apps list")
	}

	tab.cwsAppInstalled = cwsAppInstalled
	return nil
}

func (tab *youtubeTab) NameInTaskManager() string {
	if tab.cwsAppInstalled {
		return "App: " + tab.Title
	}
	return tab.ChromeTab.NameInTaskManager()
}
