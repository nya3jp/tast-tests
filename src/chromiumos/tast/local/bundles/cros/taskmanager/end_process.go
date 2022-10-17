// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package taskmanager

import (
	"context"
	"math/rand"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/taskmanager"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         EndProcess,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Verify the 'End process' button works on plugin, non-plugin and grouped tabs",
		Contacts: []string{
			"sun.tsai@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
		Timeout:      10 * time.Minute,
	})
}

type endProcessTestResources struct {
	tconn       *chrome.TestConn
	ui          *uiauto.Context
	kb          *input.KeyboardEventWriter
	taskManager *taskmanager.TaskManager
}

// EndProcess verifies the "End process" button works on plugin, non-plugin and grouped tabs.
func EndProcess(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		errors.Wrap(err, "failed to get keyboard")
	}
	defer kb.Close()

	resources := &endProcessTestResources{
		tconn:       tconn,
		ui:          uiauto.New(tconn),
		kb:          kb,
		taskManager: taskmanager.New(tconn, kb),
	}

	for _, test := range []endProcessTest{
		newNonPluginTest(),
		newPluginTest(),
		newGroupedTabsTest(),
	} {
		f := func(ctx context.Context, s *testing.State) {
			cleanupCtx := ctx
			ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
			defer cancel()

			for _, process := range test.getProcesses() {
				if err := process.Open(ctx, cr, tconn, kb); err != nil {
					s.Fatal("Failed to open the process: ", err)
				}
				defer process.Close(cleanupCtx)
				defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, test.getDescription()+"_before_closing_tab")

				if tab, ok := process.(*pluginTab); ok {
					// Under some slow network connections or DUTs, the plugin node might not be loaded instantly.
					// Therefore, give some time to wait until the target node exists.
					if err := resources.ui.WithTimeout(time.Minute).WaitUntilExists(tab.pluginNode)(ctx); err != nil {
						s.Fatal("Failed to find the plugin node: ", err)
					}
				}
			}

			if err := resources.taskManager.Open(ctx); err != nil {
				s.Fatal("Failed to open the task manager: ", err)
			}
			defer resources.taskManager.Close(cleanupCtx, tconn)
			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, test.getDescription()+"_before_closing_tm")

			if err := resources.taskManager.WaitUntilStable(ctx); err != nil {
				s.Fatal("Failed to wait until the Task Manager becomes stable: ", err)
			}

			if err := test.terminateAndVerify(ctx, resources); err != nil {
				s.Fatal("Failed to terminate the process: ", err)
			}
		}

		if !s.Run(ctx, test.getDescription(), f) {
			s.Error("Failed to run ", test.getDescription())
		}
	}
}

type endProcessTest interface {
	terminateAndVerify(ctx context.Context, res *endProcessTestResources) error
	getDescription() string
	getProcesses() []taskmanager.Process
}

type nonPluginTest struct {
	description string
	processes   []taskmanager.Process
}

func newNonPluginTest() *nonPluginTest {
	processes := []taskmanager.Process{
		taskmanager.NewChromeTabProcess("https://translate.google.com/?hl=en"),
		taskmanager.NewChromeTabProcess("https://news.ycombinator.com/news"),
		taskmanager.NewChromeTabProcess("http://lite.cnn.com/en"),
		taskmanager.NewChromeTabProcess("https://help.netflix.com/en"),
		taskmanager.NewChromeTabProcess("https://www.cbc.ca/lite/trending-news"),
	}

	return &nonPluginTest{"non_plugin_test", processes}
}

func (npt *nonPluginTest) terminateAndVerify(ctx context.Context, res *endProcessTestResources) error {
	return terminateAndVerify(ctx, npt, res)
}

func (npt *nonPluginTest) getDescription() string {
	return npt.description
}

func (npt *nonPluginTest) getProcesses() []taskmanager.Process {
	return npt.processes
}

type pluginTab struct {
	*taskmanager.ChromeTab
	pluginName string
	pluginNode *nodewith.Finder
}

func newPluginTab(url, pluginName string, pluginNode *nodewith.Finder) *pluginTab {
	return &pluginTab{
		ChromeTab:  taskmanager.NewChromeTabProcess(url),
		pluginName: pluginName,
		pluginNode: pluginNode,
	}
}

func (pTab *pluginTab) NameInTaskManager(ctx context.Context, tconn *chrome.TestConn) (string, error) {
	// Plugin name is not changed dynamically. Just return its name directly.
	return "Subframe: " + pTab.pluginName, nil
}

type pluginTest struct {
	description string
	processes   []taskmanager.Process
}

func newPluginTest() *pluginTest {
	processes := []taskmanager.Process{
		newPluginTab("https://twitter.com/i/flow/signup",
			"https://accounts.google.com/", nodewith.Name("Sign up with Google").Role(role.Button),
		),
		newPluginTab("https://www.oreilly.com",
			"https://driftt.com/", nodewith.NameStartingWith("Chat message from O'Reilly Bot:").Role(role.Button),
		),
	}

	return &pluginTest{"plugin_test", processes}
}

func (pt *pluginTest) terminateAndVerify(ctx context.Context, res *endProcessTestResources) error {
	rand.Seed(time.Now().UnixNano())
	p := pt.processes[rand.Intn(len(pt.processes))]

	tab, ok := p.(*pluginTab)
	if !ok {
		return errors.New("unexpected process")
	}

	name, err := p.NameInTaskManager(ctx, res.tconn)
	if err != nil {
		return errors.Wrap(err, "failed to obtain the process name in task manager")
	}
	testing.ContextLogf(ctx, "Terminate plugin process %q", name)
	if err := res.taskManager.TerminateProcess(name)(ctx); err != nil {
		return errors.Wrap(err, "failed to verify 'End process' button works")
	}

	if err := res.tconn.Call(ctx, nil, "async (id) => tast.promisify(chrome.tabs.update)(id, {active: true})", tab.ID); err != nil {
		return errors.Wrap(err, "failed to focus on the target tab")
	}

	return res.ui.WaitUntilGone(tab.pluginNode)(ctx)
}

func (pt *pluginTest) getDescription() string {
	return pt.description
}

func (pt *pluginTest) getProcesses() []taskmanager.Process {
	return pt.processes
}

type groupedTabsTest struct {
	description string
	processes   []taskmanager.Process
}

func newGroupedTabsTest() *groupedTabsTest {
	var processes []taskmanager.Process
	const groupedTabsAmount = 5

	for i := 0; i < groupedTabsAmount; i++ {
		tab := taskmanager.NewChromeTabProcess(chrome.NewTabURL)
		processes = append(processes, tab)
	}

	return &groupedTabsTest{"grouped_tabs_test", processes}
}

func (gtt *groupedTabsTest) terminateAndVerify(ctx context.Context, res *endProcessTestResources) error {
	return terminateAndVerify(ctx, gtt, res)
}

func (gtt *groupedTabsTest) getDescription() string {
	return gtt.description
}

func (gtt *groupedTabsTest) getProcesses() []taskmanager.Process {
	return gtt.processes
}

func terminateAndVerify(ctx context.Context, test endProcessTest, res *endProcessTestResources) error {
	rand.Seed(time.Now().UnixNano())
	n := rand.Intn(len(test.getProcesses()))
	p := test.getProcesses()[n]

	var processesToBeVerified []taskmanager.Process
	switch test.(type) {
	case *nonPluginTest:
		processesToBeVerified = append(processesToBeVerified, p)
	case *groupedTabsTest:
		for _, process := range test.getProcesses() {
			processesToBeVerified = append(processesToBeVerified, process)
		}
	default:
		return errors.New("unexpected test type")
	}

	for _, process := range processesToBeVerified {
		if status, err := process.Status(ctx, res.tconn); err != nil {
			return err
		} else if status != taskmanager.ProcessAlive {
			return errors.Errorf("expecting the tab process to be alive, but got %q", status)
		}
	}

	name, err := p.NameInTaskManager(ctx, res.tconn)
	if err != nil {
		return errors.Wrap(err, "failed to obtain the process name in task manager")
	}
	testing.ContextLogf(ctx, "Terminate process %q", name)
	if err := res.taskManager.TerminateProcess(name)(ctx); err != nil {
		return errors.Wrap(err, "failed to verify 'End process' button works")
	}

	for _, process := range processesToBeVerified {
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if status, err := process.Status(ctx, res.tconn); err != nil {
				return err
			} else if status != taskmanager.ProcessDead {
				return errors.Errorf("expecting the tab process to be dead, but got %q", status)
			}
			return nil
		}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
			return errors.Wrapf(err, "failed to verify the process %q is terminated", name)
		}
	}

	return nil
}
