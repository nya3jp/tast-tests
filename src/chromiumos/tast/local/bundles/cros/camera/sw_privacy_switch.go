// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"path/filepath"

	"chromiumos/tast/local/gtest"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     SWPrivacySwitch,
		Desc:     "Runs sw_privacy_switch_test to verify SWPrivacySwitchStreamManipulator works",
		Contacts: []string{"okuji@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

func SWPrivacySwitch(ctx context.Context, s *testing.State) {
	const gtestExecutable = "sw_privacy_switch_test"
	if _, err := gtest.New(
		gtestExecutable,
		gtest.Logfile(filepath.Join(s.OutDir(), gtestExecutable+".log")),
	).Run(ctx); err != nil {
		s.Errorf("Failed to run %v: %v", gtestExecutable, err)
	}
}
