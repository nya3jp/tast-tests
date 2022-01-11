// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package wmputils contains utility functions for wmp tests.
package wmputils

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type recordModeCondition struct {
	function action.Action
	errorMsg string
	key      string
}

// EnsureCaptureModeActivated makes sure that the capture mode is activated.
func EnsureCaptureModeActivated(tconn *chrome.TestConn, activated bool) uiauto.Action {
	return func(ctx context.Context) error {
		ac := uiauto.New(tconn)

		kb, err := input.Keyboard(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to create a keyboard")
		}

		topRow, err := input.KeyboardTopRowLayout(ctx, kb)
		if err != nil {
			return errors.Wrap(err, "failed to load the top-row layout")
		}

		screenRecordToggleButton := nodewith.ClassName("CaptureModeToggleButton").Name("Screen record")

		return testing.Poll(ctx, func(ctx context.Context) error {
			var condition recordModeCondition
			if activated {
				condition = recordModeCondition{
					function: ac.Exists(screenRecordToggleButton),
					errorMsg: "it hasn't entered record mode yet",
					key:      "Ctrl+Shift+" + topRow.SelectTask,
				}
			} else {
				condition = recordModeCondition{
					function: ac.Gone(screenRecordToggleButton),
					errorMsg: "it is still in record mode",
					key:      "Esc",
				}
			}

			if err := condition.function(ctx); err != nil {
				if err := kb.AccelAction(condition.key)(ctx); err != nil {
					return errors.Wrapf(err, "failed to press %s", condition.key)
				}
				return errors.Wrap(err, condition.errorMsg)
			}
			return nil
		}, &testing.PollOptions{
			Interval: 500 * time.Millisecond,
			Timeout:  5 * time.Second,
		})
	}
}
