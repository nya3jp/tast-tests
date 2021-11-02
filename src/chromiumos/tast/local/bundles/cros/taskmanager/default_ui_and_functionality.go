// Copyright 2022 The Chromium OS Authors. All rights reserved.
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
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify Task Manager default UI and functionality",
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

type taskManagerDefaultTestResources struct {
	tconn       *chrome.TestConn
	ui          *uiauto.Context
	kb          *input.KeyboardEventWriter
	taskManager *taskmanager.TaskManager
	processes   []taskmanager.Process
}

// DefaultUIAndFunctionality verifies Task Manager default UI and the basic functionality of "End process" button.
func DefaultUIAndFunctionality(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

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
			taskmanager.NewChromeTabProcess("https://www.cbc.ca/lite/trending-news", "CBC Lite"),
			taskmanager.NewChromeTabProcess("https://www.nytimes.com/timeswire", "The New York Times"),
			taskmanager.NewChromeTabProcess("https://help.netflix.com/en", "Netflix Help"),
			taskmanager.NewChromeTabProcess("http://lite.cnn.com/en", "CNN"),
			taskmanager.NewChromeTabProcess("https://news.ycombinator.com/news", "Hacker News"),
		},
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	for _, process := range resources.processes {
		if err := process.Open(ctx, cr, tconn); err != nil {
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

			if err := resources.taskManager.Open()(ctx); err != nil {
				s.Fatal("Failed to open Task Manager: ", err)
			}
			defer resources.taskManager.Close(cleanupSubTestCtx, tconn)

			if err := resources.taskManager.EnsureOpened(ctx); err != nil {
				s.Fatal("Failed to verify Task Manager default UI is properly displayed: ", err)
			}

			defer faillog.DumpUITreeWithScreenshotOnError(cleanupSubTestCtx, s.OutDir(), s.HasError, cr, test.getDescription())

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
type processExists struct {
	*taskManagerDefaultTestResources
	description string
}

func newProcessExistsVerifier(res *taskManagerDefaultTestResources) *processExists {
	return &processExists{res, "process_exist"}
}

func (f *processExists) getDescription() string { return f.description }

func (f *processExists) verify(ctx context.Context) error {
	if err := f.taskManager.EnsureOpened(ctx); err != nil {
		return errors.Wrap(err, "failed to find the default UI in the task manager")
	}

	for _, process := range f.processes {
		processFinder := taskmanager.FindProcess().NameStartingWith(process.NameInTaskManager())
		if err := f.ui.WaitUntilExists(processFinder)(ctx); err != nil {
			return errors.Wrap(err, "failed to find the process")
		}
		testing.ContextLogf(ctx, "Found process %q", process.NameInTaskManager())
	}

	return nil
}

// endProcessButtonEnabled defines the struct for verifying if end process button is enabled.
type endProcessButtonEnabled struct {
	*taskManagerDefaultTestResources
	description string
}

func newEndProcessButtonEnabledVerifier(res *taskManagerDefaultTestResources) *endProcessButtonEnabled {
	return &endProcessButtonEnabled{res, "end_process_button_enabled"}
}

func (f *endProcessButtonEnabled) getDescription() string { return f.description }

func (f *endProcessButtonEnabled) verify(ctx context.Context) error {
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

	// Skip the first process "Browser", which it cannot be terminated by default.
	nth := rand.Intn(len(processesInfo)-1) + 1

	testing.ContextLogf(ctx, "Select process No. %d (zero-based)", nth)
	return uiauto.Combine("ensrure the state of 'End process' button",
		ensureEndProcessButtonFocusable(f.ui, false),
		f.taskManager.SelectProcess(taskmanager.FindNthProcess(nth)),
		ensureEndProcessButtonFocusable(f.ui, true),
	)(ctx)
}

// terminateProcess defines the struct for verifying if end process button works.
type terminateProcess struct {
	*taskManagerDefaultTestResources
	description string
}

func newTerminateProcessVerifier(res *taskManagerDefaultTestResources) *terminateProcess {
	return &terminateProcess{res, "process_terminated"}
}

func (f *terminateProcess) getDescription() string { return f.description }

func (f *terminateProcess) verify(ctx context.Context) error {
	rand.Seed(time.Now().UnixNano())
	p := f.processes[rand.Intn(len(f.processes))]

	if status, err := p.GetStatus(ctx, f.tconn); err != nil {
		return err
	} else if status != taskmanager.TabComplete {
		return errors.Wrapf(err, "failed to ensure the tab status is complete, got status: %s", status)
	}

	testing.ContextLogf(ctx, "Terminate process %q", p.NameInTaskManager())
	processFinder := taskmanager.FindProcess().NameStartingWith(p.NameInTaskManager())
	if err := f.taskManager.TerminateProcess(processFinder)(ctx); err != nil {
		return errors.Wrap(err, "failed to verify 'End process' button works")
	}

	if status, err := p.GetStatus(ctx, f.tconn); err != nil {
		return err
	} else if status != taskmanager.TabUnloaded {
		return errors.Wrapf(err, "failed to ensure the tab is crashed, got status: %s", status)
	}
	testing.ContextLogf(ctx, "Process %q is terminated", p.NameInTaskManager())

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
