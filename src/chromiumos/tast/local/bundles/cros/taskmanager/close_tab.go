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
		Func: CloseTab,
		Desc: "Test the entry should be removed in task manager automatically after closing tab",
		Contacts: []string{
			"kyle.chen@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

type closeTabInTaskManagerTestResources struct {
	cr          *chrome.Chrome
	outDir      string
	tconn       *chrome.TestConn
	ui          *uiauto.Context
	kb          *input.KeyboardEventWriter
	taskManager *taskmanager.TaskManager
	processes   []*tabProcess
}

// CloseTab tests the entry should be removed in task manager automatically after closing tab.
func CloseTab(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create keyboard input: ", err)
	}
	defer kb.Close()

	resources := &closeTabInTaskManagerTestResources{
		cr:          cr,
		outDir:      s.OutDir(),
		tconn:       tconn,
		ui:          uiauto.New(tconn),
		kb:          kb,
		taskManager: taskmanager.New(tconn, kb),
		processes: []*tabProcess{
			newTabProcess("https://www.facebook.com", "Facebook"),
			newTabProcess("https://www.amazon.com", "Amazon"),
			newTabProcess("https://www.apple.com", "Apple"),
			newTabProcess("https://en.wikipedia.org/wiki/Main_Page", "Wikipedia"),
			newTabProcess("https://news.google.com", "Google News"),
			newTabProcess("https://www.youtube.com", "YouTube"),
			newTabProcess("https://help.netflix.com/en", "Netflix Help"),
			newTabProcess("https://news.ycombinator.com/news", "Hacker News"),
			newTabProcess("https://www.cbc.ca/lite/trending-news", "CBC Lite"),
			newTabProcess("https://translate.google.com/?hl=en", "Google Translate"),
		},
	}
	numberOfTabs := len(resources.processes)

	// Select half of tabs to close.
	needToClose := make([]bool, numberOfTabs)
	for idx := range needToClose {
		needToClose[idx] = idx < numberOfTabs/2
	}
	// Shuffle to make the selection randomly.
	rand.Shuffle(numberOfTabs, func(i, j int) { needToClose[i], needToClose[j] = needToClose[j], needToClose[i] })
	for idx := range resources.processes {
		resources.processes[idx].needToClose = needToClose[idx]
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	for _, tab := range resources.processes {
		if err := tab.Open(ctx, cr); err != nil {
			s.Fatal("Failed to open tab: ", err)
		}
		defer tab.Close(cleanupCtx)
	}
	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "chrome_tab_ui_dump")

	if err := checkTabsInTaskManager(ctx, resources); err != nil {
		s.Fatal("Failed to check all tabs exist in task manager: ", err)
	}

	if err := kb.Accel(ctx, "Ctrl+1"); err != nil {
		s.Fatal("Failed to switch to the first tab: ", err)
	}

	for _, tab := range resources.processes {
		if tab.needToClose {
			if err := tab.closeTab(ctx, resources.ui); err != nil {
				s.Fatal("Failed to close tab: ", err)
			}
		} else {
			if err := kb.Accel(ctx, "Ctrl+Tab"); err != nil {
				s.Fatal("Failed to switch to the next tab: ", err)
			}
		}
	}

	if err := checkTabsInTaskManager(ctx, resources); err != nil {
		s.Fatal("Failed to verify that the unclosed entry in task manager still exist: ", err)
	}
}

func checkTabsInTaskManager(ctx context.Context, resources *closeTabInTaskManagerTestResources) (retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	if err := resources.taskManager.Open()(ctx); err != nil {
		return errors.Wrap(err, "failed to launch chrome's task manager")
	}
	defer resources.taskManager.Close(cleanupCtx, resources.tconn)
	defer faillog.DumpUITreeWithScreenshotOnError(ctx, resources.outDir, func() bool { return retErr != nil }, resources.cr, "taskManager_ui_dump")

	verify := map[bool]func(*nodewith.Finder) action.Action{
		true:  resources.ui.WaitUntilGone,
		false: resources.ui.WaitUntilExists,
	}

	for _, tab := range resources.processes {
		if err := verify[tab.closed](nodewith.NameStartingWith(tab.NameInTaskManager()))(ctx); err != nil {
			return errors.Wrapf(err, "failed to check the state of %q in task manager", tab.NameInTaskManager())
		}

		if tab.closed {
			testing.ContextLogf(ctx, "%14q has verified closed", tab.NameInTaskManager())
		} else {
			testing.ContextLogf(ctx, "%14q has verified remain open", tab.NameInTaskManager())
		}
	}

	return nil
}

type tabProcess struct {
	taskmanager.Process
	name        string
	closed      bool
	needToClose bool
}

func newTabProcess(url, tabName string) *tabProcess {
	return &tabProcess{
		Process:     taskmanager.NewChromeTabProcess(url, tabName),
		name:        tabName,
		closed:      false,
		needToClose: false,
	}
}

// closeTab closes the tab and confirm closed is true.
func (tab *tabProcess) closeTab(ctx context.Context, ui *uiauto.Context) error {
	targetTab := nodewith.NameContaining(tab.name).HasClass("Tab").Ancestor(nodewith.HasClass("BrowserView"))

	if err := uiauto.Combine("close tab",
		ui.LeftClick(nodewith.Name("Close").HasClass("TabCloseButton").Ancestor(targetTab)),
		ui.WaitUntilGone(targetTab),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to click close button")
	}

	tab.closed = true

	return nil
}
