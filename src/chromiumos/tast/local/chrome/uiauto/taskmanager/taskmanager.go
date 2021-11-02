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

// TaskManager holds the resources required to operate on the Task Manager.
type TaskManager struct {
	ui  *uiauto.Context
	kb  *input.KeyboardEventWriter
	app apps.App
}

// New returns an instance of TaskManager.
func New(tconn *chrome.TestConn, kb *input.KeyboardEventWriter) *TaskManager {
	return &TaskManager{
		ui: uiauto.New(tconn),
		kb: kb,
		app: apps.App{
			ID:   "ijaigheoohcacdnplfbdimmcfldnnhdi",
			Name: "Task Manager",
		},
	}
}

// Open opens the task manager.
func (tm *TaskManager) Open() uiauto.Action {
	return tm.kb.AccelAction("Search+Esc")
}

// Close closes the task manager.
func (tm *TaskManager) Close(ctx context.Context, tconn *chrome.TestConn) {
	if err := apps.Close(ctx, tconn, tm.app.ID); err != nil {
		testing.ContextLog(ctx, "Failed to close the task manager: ", err)
	}
}

// EnsureOpened ensures the task manager is opened by checking its UI.
func (tm *TaskManager) EnsureOpened(ctx context.Context) error {
	for _, node := range []*nodewith.Finder{
		nodewith.Name("Task").HasClass("AXVirtualView").Role(role.ColumnHeader),
		nodewith.Name("Memory footprint").HasClass("AXVirtualView").Role(role.ColumnHeader),
		nodewith.Name("CPU").HasClass("AXVirtualView").Role(role.ColumnHeader),
		nodewith.Name("Network").HasClass("AXVirtualView").Role(role.ColumnHeader),
		nodewith.Name("Process ID").HasClass("AXVirtualView").Role(role.ColumnHeader),
	} {
		if err := tm.ui.WaitUntilExists(node)(ctx); err != nil {
			return errors.Wrap(err, "failed to find the column header")
		}
	}

	if topTaskInfo, err := tm.ui.Info(ctx, nodewith.HasClass("AXVirtualView").Role(role.Cell).First()); err != nil {
		return errors.Wrap(err, "failed to get the information of the top task")
	} else if topTaskInfo.Name != "Browser" {
		return errors.Errorf("expecting 'Browser' on top of the task manager, but got %s", topTaskInfo.Name)
	}

	return nil
}

// ProcessInformationCell represents the order of the column header in the task manager.
type ProcessInformationCell int

// const block defines the order of column header.
const (
	ProcessName ProcessInformationCell = iota
	Memory
	CPU
	Network
	ProcessID
	ProcessInformationCellEnd
	ProcessInformationCellCount = ProcessInformationCellEnd
)

// FindProcess returns the finder of a process node in the task manager.
func FindProcess() *nodewith.Finder { return nodewith.HasClass("AXVirtualView").Role(role.Cell) }

// FindNthProcess returns the finder of the nth row of process node in the task manager.
func FindNthProcess(nth int) *nodewith.Finder {
	return FindProcess().Nth(int(ProcessInformationCellCount)*nth + int(ProcessName))
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
		tm.ui.LeftClick(nodewith.Name("End process").HasClass("MdTextButton").Role(role.Button)),
		tm.ui.WaitUntilGone(p),
	)
}

// processFocused ensures the given process node is focused.
func (tm *TaskManager) processFocused(p *nodewith.Finder) uiauto.Action {
	return func(ctx context.Context) error {
		targetProcessInfo, err := tm.ui.Info(ctx, p)
		if err != nil {
			return errors.Wrap(err, "failed to get the information of the wanted process")
		}

		if !targetProcessInfo.State[state.Focused] {
			return errors.New("target not focused")
		}

		return nil
	}
}
