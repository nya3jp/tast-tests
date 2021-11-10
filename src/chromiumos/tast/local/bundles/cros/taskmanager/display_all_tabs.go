// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package taskmanager

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
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

	youtubeCWSAppInstalled, err := ash.ChromeAppInstalled(ctx, tconn, apps.YouTubeCWS.ID)
	if err != nil {
		s.Fatal("Failed to get Chrome Apps list: ", err)
	}

	// Expecting 3 windows, 2 tabs on the first window, 4 tabs on the second and the third window.
	browserTabs := [][]taskmanager.Process{
		{
			newChromeTabInNewWindow("https://www.facebook.com/"),
			newExtensionTab(chrome.BlankURL, "Speedtest", ui),
		}, {
			newChromeTabInNewWindow("https://www.amazon.com/"),
			taskmanager.NewChromeTabProcess("https://www.apple.com/"),
			newYoutubeTab("https://www.youtube.com/", youtubeCWSAppInstalled),
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

	if err := cws.InstallApp(ctx, cr, tconn, cws.App{Name: cwsAppName, URL: cwsAppURL}); err != nil {
		s.Fatal("Failed to install CWS app: ", err)
	}
	defer uninstallCWSExtension(cleanupCtx, cr, tconn)

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

func uninstallCWSExtension(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn) error {
	cws, err := cr.NewConn(ctx, cwsAppURL)
	if err != nil {
		return err
	}
	defer cws.Close()
	defer cws.CloseTarget(ctx)

	ui := uiauto.New(tconn)
	return uiauto.Combine("uninstall the extension from CWS",
		ui.LeftClick(nodewith.Role(role.Button).Name("Remove from Chrome").First()),
		ui.LeftClick(nodewith.Role(role.Button).Name("Remove")),
	)(ctx)
}

func newChromeTabInNewWindow(url string) *taskmanager.ChromeTab {
	tab := taskmanager.NewChromeTabProcess(url)
	tab.SetOpenInNewWindow()
	return tab
}

type extensionTab struct {
	*taskmanager.ChromeTab
	name string
	ui   *uiauto.Context
}

func newExtensionTab(url, name string, ui *uiauto.Context) *extensionTab {
	return &extensionTab{
		ChromeTab: taskmanager.NewChromeTabProcess(url),
		name:      name,
		ui:        ui,
	}
}

func newExtensionTabInNewWindow(url, name string, ui *uiauto.Context) *extensionTab {
	tab := newExtensionTab(url, name, ui)
	tab.SetOpenInNewWindow()
	return tab
}

func (tab *extensionTab) Open(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn, kb *input.KeyboardEventWriter) (retErr error) {
	if err := tab.ChromeTab.Open(ctx, cr, tconn, kb); err != nil {
		return err
	}

	browserFrame := nodewith.HasClass("BrowserFrame").Role(role.Window)
	extensionMenu := nodewith.HasClass("ExtensionsMenuView").Role(role.Window)
	return uiauto.Combine("open the extension",
		tab.ui.LeftClick(nodewith.Name("Extensions").Role(role.PopUpButton).Ancestor(browserFrame)),
		tab.ui.LeftClick(nodewith.NameStartingWith(tab.name).HasClass("ExtensionsMenuButton").Ancestor(extensionMenu)),
		tab.ui.WaitUntilExists(nodewith.Name(tab.name).Role(role.RootWebArea)),
	)(ctx)
}

func (tab *extensionTab) NameInTaskManager() string {
	return "Extension: " + tab.name
}

type youtubeTab struct {
	*taskmanager.ChromeTab
	cwsAppInstalled bool
}

func newYoutubeTab(url string, cwsAppInstalled bool) *youtubeTab {
	return &youtubeTab{
		ChromeTab:       taskmanager.NewChromeTabProcess(url),
		cwsAppInstalled: cwsAppInstalled,
	}
}

func newYoutubeTabInNewWindow(url string, cwsAppExists bool) *youtubeTab {
	tab := newYoutubeTab(url, cwsAppExists)
	tab.SetOpenInNewWindow()
	return tab
}

func (tab *youtubeTab) NameInTaskManager() string {
	if tab.cwsAppInstalled {
		return "App: " + tab.Title
	}
	return tab.ChromeTab.NameInTaskManager()
}
