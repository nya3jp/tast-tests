// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package taskmanager contains functions related to the task manager.
package taskmanager

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

var (
	// rootFinder is the finder for the Task Manager.
	rootFinder = nodewith.Name("Task Manager").HasClass("TaskManagerView")

	// EndProcessFinder is the finder for End Process button.
	EndProcessFinder = nodewith.Name("End process").Role(role.Button).FinalAncestor(rootFinder)
)

// TaskManager holds the resources required to operate on the Task Manager.
type TaskManager struct {
	ui  *uiauto.Context
	kb  *input.KeyboardEventWriter
	app apps.App
}

// New returns an instance of TaskManager.
func New(tconn *chrome.TestConn, kb *input.KeyboardEventWriter) *TaskManager {
	return &TaskManager{
		ui:  uiauto.New(tconn),
		kb:  kb,
		app: apps.TaskManager,
	}
}

// Open opens the task manager.
func (tm *TaskManager) Open(ctx context.Context) error {
	if err := tm.kb.Accel(ctx, "Search+Esc"); err != nil {
		return errors.Wrap(err, "failed to press key to open the Task Manager")
	}

	return tm.ui.WaitUntilExists(rootFinder)(ctx)
}

// WaitUntilStable waits task manager UI to become stable.
func (tm *TaskManager) WaitUntilStable(ctx context.Context) error {
	processRows := nodewith.HasClass("AXVirtualView").Role(role.Row).State(state.Multiselectable, true)
	const threshold = 3
	var contentsLoaded int
	var numberOfRows int

	return testing.Poll(ctx, func(ctx context.Context) error {
		processesInfo, err := tm.ui.NodesInfo(ctx, processRows)
		if err != nil {
			return testing.PollBreak(err)
		}

		if numberOfRows == len(processesInfo) {
			contentsLoaded++
			if contentsLoaded >= threshold {
				return nil
			}
			return errors.New("contents have stopped loading but not stabilized yet")
		}

		numberOfRows = len(processesInfo)
		contentsLoaded = 0
		return errors.New("contents have not stopped loading")
	}, &testing.PollOptions{Interval: 3 * time.Second, Timeout: time.Minute})
}

// Close closes the task manager.
func (tm *TaskManager) Close(ctx context.Context, tconn *chrome.TestConn) error {
	return apps.Close(ctx, tconn, tm.app.ID)
}

// FindProcess returns the finder of a process node in the task manager.
func FindProcess() *nodewith.Finder { return nodewith.HasClass("AXVirtualView").Role(role.Cell) }

// FindNthProcess returns the finder of the nth row and the first column of the process node in the task manager.
func FindNthProcess(ctx context.Context, ui *uiauto.Context, nth int) (*nodewith.Finder, error) {
	columnHeader := nodewith.HasClass("AXVirtualView").Role(role.ColumnHeader)
	columns, err := ui.NodesInfo(ctx, columnHeader)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the information of the column header")
	}
	return FindProcess().Nth(len(columns) * nth), nil
}

// SelectProcess selects the specific process in the task manager.
func (tm *TaskManager) SelectProcess(p *nodewith.Finder) uiauto.Action {
	return func(ctx context.Context) error {
		return testing.Poll(ctx, func(c context.Context) error {
			// Press down key to scroll down the task manager list until the chosen process entry is focused.
			if err := tm.kb.AccelAction("Down")(ctx); err != nil {
				return testing.PollBreak(err)
			}
			return tm.processFocused(p)(ctx)
		}, &testing.PollOptions{Timeout: time.Minute})
	}
}

// TerminateProcess terminates the process.
func (tm *TaskManager) TerminateProcess(p *nodewith.Finder) uiauto.Action {
	return uiauto.Combine("end process",
		tm.SelectProcess(p),
		tm.ui.LeftClick(EndProcessFinder),
	)
}

// processFocused ensures the given process node is focused.
func (tm *TaskManager) processFocused(p *nodewith.Finder) uiauto.Action {
	return tm.ui.RetrySilently(3, func(ctx context.Context) error {
		targetProcessInfo, err := tm.ui.WithTimeout(time.Second).Info(ctx, p)
		if err != nil {
			return errors.Wrap(err, "failed to get the information of the target process")
		}

		if !targetProcessInfo.State[state.Focused] {
			return errors.New("target not focused")
		}

		return nil
	})
}
