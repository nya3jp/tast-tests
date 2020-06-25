// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"chromiumos/tast/local/chrome/metrics"
)


type TabCrashChecker struct {
	recorder *Recorder
}

func NewTabCrashChecker(ctx context.Context, tconn *chrome.TestConn) (*TabCrashChecker, error) {
	sadHistRecorder, err := metrics.StartRecorder(ctx, tconn, "Tabs.SadTab.CrashCreated",
		"Tabs.SadTab.OomCreated", "Tabs.SadTab.KillCreated.OOM", "Tabs.SadTab.KillCreated")
	if err != nil {
		return (nil, errors.Wrap(err, "failed to start histogram recorder for sad tabs")
	}
	r := &TabCrashChecker{
		recorder: sadHistRecorder
	}
	return (r, nil)
}

func (checker *TabCrashChecker) Check() error {
	sadDiffs, err := sadHistRecorder.Histogram(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get diff of histograms: ", err)
	}
	// Check the sadDiffs and fail if any histogram has non-zero num.
	for _, h := range sadDiffs {
		if h.Sum != 0 {
			s.Fatalf("Tab renderer crashed. Sad tab showed up (histogram %s).", h.Name)
		}
	}
}