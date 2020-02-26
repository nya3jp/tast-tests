// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func exitIfShown(ctx context.Context, tconn *chrome.TestConn, appID string) error {
	if visible, err := ash.AppShown(ctx, tconn, appID); err != nil {
		return err
	} else if !visible {
		return nil
	}
	return apps.Close(ctx, tconn, appID)
}

func findNewShelfItem(before []*ash.ShelfItem, after []*ash.ShelfItem) (string, error) {
	if len(before) == len(after) {
		return "", errors.New("no new shelf item")
	}
	if len(before)+1 != len(after) {
		return "", errors.Errorf("item number mismatch, got %d wanted %d", len(after), len(before)+1)
	}
	beforeMap := map[string]bool{}
	for _, beforeItem := range before {
		beforeMap[beforeItem.AppID] = true
	}
	for _, afterItem := range after {
		if !beforeMap[afterItem.AppID] {
			return afterItem.AppID, nil
		}
	}
	return "", errors.New("could not find the new shelf item")
}

// LaunchGUIApp runs the given command, which is meant to be a crostini
// application with a GUI, and returns:
//  - A string, containing the ID of the app that was ran (i.e., a handle which
//    can be used to inspect/close the app).
//  - A callback which can be executed to close the application. Users of this
//    function should immediately defer the callback if one is returned.
//  - An error, which indicates something went wrong, or nil otherwise.
func LaunchGUIApp(ctx context.Context, tconn *chrome.TestConn, cmd *testexec.Cmd) (string, func(), error) {
	beforeItems, err := ash.ShelfItems(ctx, tconn)
	if err != nil {
		return "", func() {}, err
	}
	if err := cmd.Start(); err != nil {
		return "", func() {}, errors.Wrapf(err, "failed to start %q", shutil.EscapeSlice(cmd.Args))
	}
	var newID string
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if currentItems, err := ash.ShelfItems(ctx, tconn); err != nil {
			return err
		} else if newID, err = findNewShelfItem(beforeItems, currentItems); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		cmd.Kill()
		cmd.Wait(testexec.DumpLogOnError)
		return "", func() {}, err
	}
	return newID, func() {
		exitIfShown(ctx, tconn, newID)
		cmd.Wait(testexec.DumpLogOnError)
	}, nil
}
