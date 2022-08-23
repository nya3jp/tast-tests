// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package deskscuj

import (
	"context"
	"fmt"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/input"
)

// deskSwitchWorkflow represents a workflow for switching between desks.
// |run| switches to the "next" desk, following the pattern:
// Desk 1 -> Desk 2 -> Desk 3 -> Desk 4 -> Desk 3 -> Desk 2-> Desk 1.
// |run| returns the expected index of the newly activated desk.
type deskSwitchWorkflow struct {
	name        string
	description string
	run         func(context.Context) (int, error)
}

// getKeyboardSearchBracketWorkflow returns the workflow for switching
// between desks using Search+[ and Search+]. Before calling the run
// function for the first time, the desk 0 must be active.
func getKeyboardSearchBracketWorkflow(tconn *chrome.TestConn, kw *input.KeyboardEventWriter, numDesks int) deskSwitchWorkflow {
	nextDeskIncrement := -1
	activeDesk := 0
	return deskSwitchWorkflow{
		name:        "Search+] and Search+[",
		description: "Cycle through desks using Search+] and Search+[",
		run: func(ctx context.Context) (int, error) {
			if activeDesk == 0 || activeDesk == numDesks-1 {
				nextDeskIncrement *= -1
			}
			activeDesk += nextDeskIncrement

			direction := "Search+]"
			if nextDeskIncrement < 0 {
				direction = "Search+["
			}

			return activeDesk, kw.Accel(ctx, direction)
		},
	}
}

// getKeyboardSearchNumberWorkflow returns the workflow for switching
// between desks using Search+Shift+Number. Before calling run for the
// first time, desk 0 must be active.
func getKeyboardSearchNumberWorkflow(tconn *chrome.TestConn, kw *input.KeyboardEventWriter, numDesks int) deskSwitchWorkflow {
	nextDeskIncrement := -1
	currDesk := 0
	return deskSwitchWorkflow{
		name:        "Search+Shift+Number",
		description: "Cycle through desks using Search+Shift+Number",
		run: func(ctx context.Context) (int, error) {
			if currDesk == 0 || currDesk == numDesks-1 {
				nextDeskIncrement *= -1
			}
			currDesk += nextDeskIncrement

			// Shortcuts are 1-indexed, so offset currDesk by 1.
			return currDesk, kw.Accel(ctx, fmt.Sprintf("Shift+Search+%d", currDesk+1))
		},
	}
}

// getOverviewWorkflow returns the workflow for switching between desks
// by entering overview mode and selecting the next desk. Before
// calling run for the first time, desk 0 must be active.
func getOverviewWorkflow(tconn *chrome.TestConn, ac *uiauto.Context, setOverviewModeAndWait action.Action, numDesks int) deskSwitchWorkflow {
	nextDeskIncrement := -1
	activeDesk := 0
	return deskSwitchWorkflow{
		name:        "Overview",
		description: "Cycle through desks using overview mode",
		run: func(ctx context.Context) (int, error) {
			if activeDesk == 0 || activeDesk == numDesks-1 {
				nextDeskIncrement *= -1
			}
			activeDesk += nextDeskIncrement

			if err := setOverviewModeAndWait(ctx); err != nil {
				return 0, errors.Wrap(err, "failed to enter overview mode")
			}

			desksInfo, err := ash.FindDeskMiniViews(ctx, ac)
			if err != nil {
				return 0, errors.Wrap(err, "failed to get desk previews")
			}

			if currNumDesks := len(desksInfo); currNumDesks != numDesks {
				return 0, errors.Wrapf(err, "unexpected number of open desks: got %d, expected %d", currNumDesks, numDesks)
			}

			if err := mouse.Click(tconn, desksInfo[activeDesk].Location.CenterPoint(), mouse.LeftButton)(ctx); err != nil {
				return 0, errors.Wrapf(err, "failed to click on the desk preview for desk %d", activeDesk)
			}

			return activeDesk, nil
		},
	}
}
