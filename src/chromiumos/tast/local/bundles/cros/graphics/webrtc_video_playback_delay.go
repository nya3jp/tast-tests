// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"time"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: WebRTCVideoPlaybackDelay,
		Desc: "Runs WebRtcVideoDisplayPerfBrowserTest to get performance numbers",
		Contacts: []string{"mcasas@chromium.org", "chromeos-gfx@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
	})
}

func WebRTCVideoPlaybackDelay(ctx context.Context, s *testing.State) {
	const exec = "browser_tests"
	cmd := testexec.CommandContext(ctx, exec,
		"--gtest_filter=*WebRtcVideoDisplayPerfBrowserTest*",
		"--run-manual")

	if err := cmd.Run(); err != nil {
		s.Errorf("Failed to run %s: %v", exec, err)
	}
}
