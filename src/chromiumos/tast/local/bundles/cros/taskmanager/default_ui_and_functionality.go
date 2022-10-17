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
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/local/chrome/uiauto/taskmanager"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DefaultUIAndFunctionality,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Verify Task Manager default UI and functionality",
		Contacts: []string{
			"sun.tsai@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
		Timeout:      5 * time.Minute,
	})
}

type taskManagerDefaultTestResources struct {
	tconn       *chrome.TestConn
	ui          *uiauto.Context
	kb          *input.KeyboardEventWriter
	taskManager *taskmanager.TaskManager
	processes   []taskmanager.Process
}

// DefaultUIAndFunctionality verifies Task Manager default UI and the basic functionality of "End process" button.
func DefaultUIAndFunctionality(ctx context.Context, s *testing.State) {
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

	resources := &taskManagerDefaultTestResources{
		tconn:       tconn,
		ui:          uiauto.New(tconn),
		kb:          kb,
		taskManager: taskmanager.New(tconn, kb),
		processes: []taskmanager.Process{
			taskmanager.NewChromeTabProcess("https://www.cbc.ca/lite/trending-news"),
			taskmanager.NewChromeTabProcess("https://translate.google.com/?hl=en"),
			taskmanager.NewChromeTabProcess("https://help.netflix.com/en"),
			taskmanager.NewChromeTabProcess("http://lite.cnn.com/en"),
			taskmanager.NewChromeTabProcess("https://news.ycombinator.com/news"),
		},
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	for _, process := range resources.processes {
		if err := process.Open(ctx, cr, tconn, kb); err != nil {
			s.Fatal("Failed to open process: ", err)
		}
		defer process.Close(cleanupCtx)
	}

	for _, test := range []taskManagerBasicFunctionality{
		newProcessExistsVerifier(resources),
		newEndProcessButtonEnabledVerifier(resources),
		newTerminateProcessVerifier(resources),
	} {
		f := func(ctx context.Context, s *testing.State) {
			cleanupSubTestCtx := ctx
			ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
			defer cancel()

			if err := resources.taskManager.Open(ctx); err != nil {
				s.Fatal("Failed to open Task Manager: ", err)
			}
			defer resources.taskManager.Close(cleanupSubTestCtx, tconn)
			defer faillog.DumpUITreeWithScreenshotOnError(cleanupSubTestCtx, s.OutDir(), s.HasError, cr, test.getDescription())

			if err := resources.taskManager.WaitUntilStable(ctx); err != nil {
				s.Fatal("Failed to wait until the Task Manager becomes stable: ", err)
			}

			if err := test.verify(ctx); err != nil {
				s.Fatal("Failed to verify Task Manager basic functionality: ", err)
			}
		}

		if !s.Run(ctx, test.getDescription(), f) {
			s.Errorf("Failed to complete test: %q", test.getDescription())
		}
	}
}

type taskManagerBasicFunctionality interface {
	getDescription() string
	verify(ctx context.Context) error
}

// processExists defines the struct for verifying process exists.
type processExistsVerifier struct {
	*taskManagerDefaultTestResources
	description string
}

func newProcessExistsVerifier(res *taskManagerDefaultTestResources) *processExistsVerifier {
	return &processExistsVerifier{res, "process_exist"}
}

func (f *processExistsVerifier) getDescription() string { return f.description }

func (f *processExistsVerifier) verify(ctx context.Context) error {
	for _, node := range []*nodewith.Finder{
		nodewith.Name("Task").HasClass("AXVirtualView").Role(role.ColumnHeader),
		nodewith.Name("Memory footprint").HasClass("AXVirtualView").Role(role.ColumnHeader),
		nodewith.Name("CPU").HasClass("AXVirtualView").Role(role.ColumnHeader),
		nodewith.Name("Network").HasClass("AXVirtualView").Role(role.ColumnHeader),
		nodewith.Name("Process ID").HasClass("AXVirtualView").Role(role.ColumnHeader),
	} {
		if err := f.ui.WaitUntilExists(node)(ctx); err != nil {
			return errors.Wrap(err, "failed to find the default column header")
		}
	}

	if topTaskInfo, err := f.ui.Info(ctx, nodewith.HasClass("AXVirtualView").Role(role.Cell).First()); err != nil {
		return errors.Wrap(err, "failed to get the information of the top task")
	} else if topTaskInfo.Name != "Browser" {
		return errors.Errorf("expecting 'Browser' on top of the task manager, but got %s", topTaskInfo.Name)
	}

	for _, process := range f.processes {
		name, err := process.NameInTaskManager(ctx, f.tconn)
		if err != nil {
			return errors.Wrap(err, "failed to obtain the process name in task manager")
		}
		processFinder := taskmanager.FindProcess().Name(name)
		if err := f.ui.WaitUntilExists(processFinder)(ctx); err != nil {
			return errors.Wrap(err, "failed to find the process")
		}
		testing.ContextLogf(ctx, "Found process %q", name)
	}

	return nil
}

// endProcessButtonEnabled defines the struct for verifying if end process button is enabled.
type endProcessButtonEnabledVerifier struct {
	*taskManagerDefaultTestResources
	description string
}

func newEndProcessButtonEnabledVerifier(res *taskManagerDefaultTestResources) *endProcessButtonEnabledVerifier {
	return &endProcessButtonEnabledVerifier{res, "end_process_button_enabled"}
}

func (f *endProcessButtonEnabledVerifier) getDescription() string { return f.description }

func (f *endProcessButtonEnabledVerifier) verify(ctx context.Context) error {
	processRows := nodewith.HasClass("AXVirtualView").Role(role.Row).State(state.Multiselectable, true)
	processesInfo, err := f.ui.NodesInfo(ctx, processRows)
	if err != nil {
		return errors.Wrap(err, "failed to get the information of the processes")
	}

	// Expecting the process "Browser" and other processes by default.
	// If there is only the process "Browser", then couldn't verify if "End process" button enabled.
	if len(processesInfo) < 2 {
		return errors.New("expecting at least two processes")
	}

	rand.Seed(time.Now().UnixNano())
	numberOfRows := len(processesInfo)
	// Skip the first process "Browser", which it cannot be terminated by default.
	nth := rand.Intn(numberOfRows-1) + 1
	process, err := taskmanager.FindNthProcess(ctx, f.ui, nth)
	if err != nil {
		return errors.Wrap(err, "failed to find the nth process")
	}

	info, err := f.ui.Info(ctx, process)
	if err != nil {
		return errors.Wrap(err, "failed to obtain the information of the process")
	}

	testing.ContextLogf(ctx, "Select process No. %d (zero-based), %q", nth, info.Name)
	return uiauto.Combine("ensure the state of 'End process' button",
		ensureEndProcessButtonFocusable(f.ui, false),
		f.taskManager.SelectProcess(info.Name),
		ensureEndProcessButtonFocusable(f.ui, true),
	)(ctx)
}

// terminateProcess defines the struct for verifying if end process button works.
type terminateProcessVerifier struct {
	*taskManagerDefaultTestResources
	description string
}

func newTerminateProcessVerifier(res *taskManagerDefaultTestResources) *terminateProcessVerifier {
	return &terminateProcessVerifier{res, "process_terminated"}
}

func (f *terminateProcessVerifier) getDescription() string { return f.description }

func (f *terminateProcessVerifier) verify(ctx context.Context) error {
	rand.Seed(time.Now().UnixNano())
	p := f.processes[rand.Intn(len(f.processes))]

	if status, err := p.Status(ctx, f.tconn); err != nil {
		return err
	} else if status != taskmanager.ProcessAlive {
		return errors.Errorf("expecting the tab process to be alive, but got %q", status)
	}

	name, err := p.NameInTaskManager(ctx, f.tconn)
	if err != nil {
		return errors.Wrap(err, "failed to obtain the process name in task manager")
	}
	testing.ContextLogf(ctx, "Terminate process %q", name)
	if err := f.taskManager.TerminateProcess(name)(ctx); err != nil {
		return errors.Wrap(err, "failed to verify 'End process' button works")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if status, err := p.Status(ctx, f.tconn); err != nil {
			return err
		} else if status != taskmanager.ProcessDead {
			return errors.Errorf("expecting the tab process to be dead, but got %q", status)
		}
		return nil
	}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
		return errors.Wrapf(err, "failed to verify the process %q is terminated", name)
	}

	return nil
}

// ensureEndProcessButtonFocusable ensures the "End process" button is enabled or disabled.
func ensureEndProcessButtonFocusable(ui *uiauto.Context, focusable bool) uiauto.Action {
	return func(ctx context.Context) error {
		endProcessBtn := nodewith.Name("End process").HasClass("MdTextButton").Role(role.Button)
		btnInfo, err := ui.Info(ctx, endProcessBtn)
		if err != nil {
			return errors.Wrap(err, "failed to get the information of 'End process' button")
		}

		if btnInfo.State[state.Focusable] != focusable {
			return errors.Errorf("expecting 'End process' button to be %t, but got %t", focusable, btnInfo.State[state.Focusable])
		}

		return nil
	}
}
