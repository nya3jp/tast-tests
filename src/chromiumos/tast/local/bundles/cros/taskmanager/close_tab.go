// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package taskmanager

import (
	"context"
	"math/rand"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/taskmanager"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CloseTab,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Test the entry should be removed in task manager automatically after closing tab",
		Contacts: []string{
			"sun.tsai@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

type closeTabTestResources struct {
	cr          *chrome.Chrome
	outDir      string
	tconn       *chrome.TestConn
	ui          *uiauto.Context
	taskManager *taskmanager.TaskManager
	processes   []taskmanager.Process
}

// CloseTab tests the entry should be removed in task manager automatically after closing tab.
func CloseTab(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create keyboard input: ", err)
	}
	defer kb.Close()

	resources := &closeTabTestResources{
		cr:          cr,
		outDir:      s.OutDir(),
		tconn:       tconn,
		ui:          uiauto.New(tconn),
		taskManager: taskmanager.New(tconn, kb),
		processes: []taskmanager.Process{
			newBrowserTabInCloseTabTest("https://www.facebook.com", "Facebook"),
			newBrowserTabInCloseTabTest("https://www.amazon.com", "Amazon"),
			newBrowserTabInCloseTabTest("https://www.apple.com", "Apple"),
			newBrowserTabInCloseTabTest("https://en.wikipedia.org/wiki/Main_Page", "Wikipedia"),
			newBrowserTabInCloseTabTest("https://news.google.com", "Google News"),
			newBrowserTabInCloseTabTest("https://www.youtube.com", "YouTube"),
			newBrowserTabInCloseTabTest("https://help.netflix.com/en", "Netflix Help"),
			newBrowserTabInCloseTabTest("https://news.ycombinator.com/news", "Hacker News"),
			newBrowserTabInCloseTabTest("https://www.cbc.ca/lite/trending-news", "CBC Lite"),
			newBrowserTabInCloseTabTest("https://translate.google.com/?hl=en", "Google Translate"),
		},
	}
	numberOfTabs := len(resources.processes)

	// Select half of tabs to close.
	needToClose := make([]bool, numberOfTabs)
	for idx := range needToClose {
		needToClose[idx] = idx < numberOfTabs/2
	}
	// Shuffle to make the selection randomly to avoid accessing some same websites every time.
	rand.Shuffle(numberOfTabs, func(i, j int) { needToClose[i], needToClose[j] = needToClose[j], needToClose[i] })
	for idx, process := range resources.processes {
		if tab, ok := process.(*browserTabInCloseTabTest); ok {
			tab.needToClose = needToClose[idx]
		}
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	for _, process := range resources.processes {
		if err := process.Open(ctx, cr, tconn, kb); err != nil {
			s.Fatal("Failed to open tab: ", err)
		}
		defer process.Close(cleanupCtx)
	}
	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "chrome_tab_ui_dump")

	if err := checkTabsInTaskManager(ctx, resources); err != nil {
		s.Fatal("Failed to check all tabs exist in task manager: ", err)
	}

	for _, process := range resources.processes {
		if tab, ok := process.(*browserTabInCloseTabTest); ok && tab.needToClose {
			targetTab := nodewith.Name(tab.Title).HasClass("Tab").Ancestor(nodewith.HasClass("BrowserView"))
			if err := uiauto.Combine("active the target tab and close it",
				tab.active(tconn),
				resources.ui.LeftClick(nodewith.Name("Close").HasClass("TabCloseButton").Ancestor(targetTab)),
				resources.ui.WaitUntilGone(targetTab),
			)(ctx); err != nil {
				s.Fatal("Failed to complete the actions: ", err)
			}
			tab.closed = true
		}
	}

	s.Log("Check the tabs in the task manager again to verify the result is expected")
	if err := checkTabsInTaskManager(ctx, resources); err != nil {
		s.Fatal("Failed to check the state of tabs: ", err)
	}
}

func checkTabsInTaskManager(ctx context.Context, resources *closeTabTestResources) (retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	if err := resources.taskManager.Open(ctx); err != nil {
		return errors.Wrap(err, "failed to launch the task manager")
	}
	defer resources.taskManager.Close(cleanupCtx, resources.tconn)
	defer faillog.DumpUITreeWithScreenshotOnError(ctx, resources.outDir, func() bool { return retErr != nil }, resources.cr, "taskManager_ui_dump")

	verify := map[bool]func(*nodewith.Finder) action.Action{
		true:  resources.ui.WaitUntilGone,
		false: resources.ui.WaitUntilExists,
	}

	for _, process := range resources.processes {
		if tab, ok := process.(*browserTabInCloseTabTest); ok {
			if err := verify[tab.closed](nodewith.Name(process.NameInTaskManager()))(ctx); err != nil {
				return errors.Wrapf(err, "failed to check the state of %q in task manager", process.NameInTaskManager())
			}

			if tab.closed {
				testing.ContextLogf(ctx, "%q is closed", process.NameInTaskManager())
			} else {
				testing.ContextLogf(ctx, "%q is opened", process.NameInTaskManager())
			}
		}
	}

	return nil
}

type browserTabInCloseTabTest struct {
	*taskmanager.ChromeTab
	closed      bool
	needToClose bool
}

func newBrowserTabInCloseTabTest(url, tabName string) *browserTabInCloseTabTest {
	return &browserTabInCloseTabTest{
		ChromeTab:   taskmanager.NewChromeTabProcess(url),
		closed:      false,
		needToClose: false,
	}
}

func (tab *browserTabInCloseTabTest) active(tconn *chrome.TestConn) uiauto.Action {
	return func(ctx context.Context) error {
		return tconn.Call(ctx, nil, "async (id) => tast.promisify(chrome.tabs.update)(id, {active: true})", tab.ID)
	}
}
