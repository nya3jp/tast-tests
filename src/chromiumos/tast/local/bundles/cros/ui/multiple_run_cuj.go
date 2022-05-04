// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/ui/cujrecorder"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MultipleRunCUJ,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Calls cujrecorder.Recorder.Run multiple times",
		Contacts:     []string{"amusbach@chromium.org", "chromeos-perfmetrics-eng@google.com"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

func MultipleRunCUJ(ctx context.Context, s *testing.State) {
	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	recorder, err := cujrecorder.NewRecorder(ctx, s.FixtValue().(chrome.HasChrome).Chrome(), nil, cujrecorder.RecorderOptions{}, cujrecorder.MetricConfigs()...)
	if err != nil {
		s.Fatal("Failed to create the recorder: ", err)
	}
	defer recorder.Close(cleanupCtx)

	testScenario := action.Sleep(16 * time.Second)
	for _, ordinal := range []string{"first", "second", "third"} {
		if err := recorder.Run(ctx, testScenario); err != nil {
			s.Errorf("Failed to conduct the performance measurement the %s time: %v", ordinal, err)
		}
	}
}
