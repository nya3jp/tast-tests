// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/metrics"
)

// TabCrashChecker is used to check if any Chrome tab is crashed during CUJ test.
type TabCrashChecker struct {
	recorder *metrics.Recorder
}

// NewTabCrashChecker creates a TabCrashChecker and starts recording tab-crash metrics.
func NewTabCrashChecker(ctx context.Context, tconn *chrome.TestConn) (*TabCrashChecker, error) {
	sadHistRecorder, err := metrics.StartRecorder(ctx, tconn, "Tabs.SadTab.CrashCreated",
		"Tabs.SadTab.OomCreated", "Tabs.SadTab.KillCreated.OOM", "Tabs.SadTab.KillCreated")
	if err != nil {
		return nil, errors.Wrap(err, "failed to start histogram recorder for sad tabs")
	}
	r := &TabCrashChecker{
		recorder: sadHistRecorder,
	}
	return r, nil
}

// Check checks if there is any tab crash after the TabCrashChecker was created.
func (checker *TabCrashChecker) Check(ctx context.Context, tconn *chrome.TestConn) error {
	sadDiffs, err := checker.recorder.Histogram(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get diff of histograms")
	}
	// Check the sadDiffs and return errors if any histogram has non-zero num.
	for _, h := range sadDiffs {
		if h.Sum != 0 {
			return errors.New("Sad tab showed up (histogram " + h.Name + ").")
		}
	}
	return nil
}