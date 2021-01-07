// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package perf

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
)

// DisplaySmoothnessTracker helps to start/stop display smoothness tracking.
type DisplaySmoothnessTracker struct {
	// Ids of the displays that have display smoothness tracked.
	displayIDs map[string]bool
}

// DisplayFrameData holds the collected display frame data.
type DisplayFrameData struct {
	FramesExpected int   `json:"framesExpected"`
	FramesProduced int   `json:"framesProduced"`
	JankCount      int   `json:"jankCount"`
	Throughput     []int `json:"throughput"`
}

// displayIDString returns a string representing the given display id.
func displayIDString(displayID string) string {
	if displayID == "" {
		return "primary display"
	}
	return displayID
}

// Close ensures all started tracking is stopped.
func (t *DisplaySmoothnessTracker) Close(ctx context.Context, tconn *chrome.TestConn) error {
	var firstErr error
	for displayID := range t.displayIDs {
		_, err := t.Stop(ctx, tconn, displayID)
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Start starts tracking for the given display id. Primary display is used
// if the given display id is empty.
func (t *DisplaySmoothnessTracker) Start(ctx context.Context, tconn *chrome.TestConn, displayID string) error {
	_, found := t.displayIDs[displayID]
	if found {
		return errors.Errorf("display smoothness already tracked for %q", displayIDString(displayID))
	}

	err := tconn.Call(ctx, nil,
		`tast.promisify(chrome.autotestPrivate.startSmoothnessTracking)`, displayID)
	if err != nil {
		return err
	}

	t.displayIDs[displayID] = true
	return nil
}

// Stop stops tracking for the given display id and report the smoothness
// since the relevant Start() call. Primary display is used if the given display
// id is empty.
func (t *DisplaySmoothnessTracker) Stop(ctx context.Context, tconn *chrome.TestConn, displayID string) (*DisplayFrameData, error) {
	_, found := t.displayIDs[displayID]
	if !found {
		return nil, errors.Errorf("display smoothness not tracked for %q", displayIDString(displayID))
	}

	var dsData DisplayFrameData
	err := tconn.Call(ctx, &dsData,
		`tast.promisify(chrome.autotestPrivate.stopSmoothnessTracking)`, displayID)
	if err != nil {
		return nil, err
	}

	delete(t.displayIDs, displayID)
	return &dsData, nil
}

// NewDisplaySmoothnessTracker creates a DisplaySmoothnessTracker.
func NewDisplaySmoothnessTracker() *DisplaySmoothnessTracker {
	return &DisplaySmoothnessTracker{
		displayIDs: map[string]bool{},
	}
}
