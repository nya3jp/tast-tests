// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package taskmanager

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
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
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get the connection to the test API: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	// Expecting 3 windows, 2 tabs on the first window, 4 tabs on the second and the third window.
	browserTabs := [][]taskmanager.Process{
		{
			newBrowserTabInNewWindow(chrome.BlankURL, "Speedtest by Ookla", withPlugin),
			newBrowserTab("https://www.facebook.com/", "Facebook", withoutPlugin),
		}, {
			newBrowserTabInNewWindow("https://www.amazon.com/", "Amazon", imageBased),
			newBrowserTab("https://www.apple.com/", "Apple", imageBased),
			newBrowserTab("https://www.walmart.com/", "Walmart", imageBased),
			newBrowserTab("https://www.instagram.com/", "Instagram", imageBased),
		}, {
			newBrowserTabInNewWindow("https://en.wikipedia.org/wiki/Main_Page", "Wikipedia", nonImageBased),
			newBrowserTab("https://news.google.com/", "Google News", nonImageBased),
			newBrowserTab("https://news.ycombinator.com/news", "Hacker News", nonImageBased),
			newBrowserTab("https://www.cbc.ca/lite/trending-news", "CBC Lite", nonImageBased),
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

	ui := uiauto.New(tconn)
	for _, browserWindow := range browserTabs {
		for _, process := range browserWindow {
			if err := ui.WaitUntilExists(taskmanager.FindProcess().NameStartingWith(process.NameInTaskManager()))(ctx); err != nil {
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

type pageType int

const (
	withPlugin pageType = iota
	withoutPlugin
	imageBased
	nonImageBased
)

type browserTab struct {
	*taskmanager.ChromeTab
	pageType pageType
}

func newBrowserTab(url, name string, pageType pageType) *browserTab {
	return &browserTab{
		ChromeTab: taskmanager.NewChromeTabProcess(url, name),
		pageType:  pageType,
	}
}

func newBrowserTabInNewWindow(url, name string, pageType pageType) *browserTab {
	tab := newBrowserTab(url, name, pageType)
	tab.SetOpenInNewWindow()
	return tab
}

func (tab *browserTab) NameInTaskManager() string {
	switch tab.pageType {
	case withPlugin:
		return `Extension: ` + tab.Name
	case withoutPlugin, imageBased, nonImageBased:
		return tab.ChromeTab.NameInTaskManager()
	default:
		return tab.ChromeTab.NameInTaskManager()
	}
}
