// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/testing"
)

// GetExternalDisplay to get display with attribute is not internal
func GetExternalDisplay(ctx context.Context, s *testing.State, tconn *chrome.TestConn) (*display.Info, error) {
	return display.FindInfo(ctx, tconn, func(info *display.Info) bool {
		return !info.IsInternal
	})
}

// GetInternalAndExternalDisplays refer to multi_display.go
func GetInternalAndExternalDisplays(ctx context.Context, tconn *chrome.TestConn) (result DisplayLayout, err error) {
	infos, err := display.GetInfo(ctx, tconn)
	if err != nil {
		return result, err
	}

	var foundInt, foundExt bool
	for _, info := range infos {
		if info.IsInternal {
			result.Internal = info
			foundInt = true
		} else if !foundExt {
			// Get the first external display info.
			result.External = info
			foundExt = true
		}
	}

	if !foundInt || !foundExt {
		err = errors.Wrap(err, "not enough displays: need at least one internal display and one external display")
		return result, err
	}

	return result, err
}

// EnsureDisplayIsPrimary if the display is not primary, then set properties & check
func EnsureDisplayIsPrimary(ctx context.Context, s *testing.State, tconn *chrome.TestConn, disp *display.Info) error {
	// check prop at first
	if disp.IsPrimary == false {

		s.Logf("Setting display [%s,%s] to be primary", disp.ID, disp.Name)

		// set the display to primary
		isPrimary := true
		if err := display.SetDisplayProperties(ctx, tconn, disp.ID, display.DisplayProperties{IsPrimary: &isPrimary}); err != nil {
			return errors.Wrap(err, "failed to make internal display become primary")
		}

		// retry in 5s
		if err := testing.Poll(ctx, func(ctx context.Context) error {

			// get primary info to compare
			primaryInfo, err := display.GetPrimaryInfo(ctx, tconn)
			if err != nil {
				return errors.Wrap(err, "failed to get primary display info ")
			}

			// check prop in the end
			if primaryInfo.ID != disp.ID {
				return errors.New("failed to set want display to be primary: ")
			}

			return nil

		}, &testing.PollOptions{Timeout: 5 * time.Second}); err != nil {
			return err
		}

		// delay for display response
		testing.Sleep(ctx, 5*time.Second)

	}
	return nil
}
