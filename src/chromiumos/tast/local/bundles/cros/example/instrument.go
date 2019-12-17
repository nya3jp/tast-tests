// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Instrument,
		Desc:         "Runs Android instrumentation tests",
		Attr:         []string{"disabled"},
		SoftwareDeps: []string{"chrome", "android_all_both"},
		Timeout:      3 * time.Minute,
		Data:         []string{"instrument.apk"},
		Pre:          arc.Booted(),
	})
}

func Instrument(ctx context.Context, s *testing.State) {
	a := s.PreValue().(arc.PreData).ARC

	s.Log("Installing APK")
	if err := a.Install(ctx, s.DataPath("instrument.apk")); err != nil {
		s.Fatal("Failed to install APK: ", err)
	}

	s.Log("Running test")
	f, err := os.Create(filepath.Join(s.OutDir(), "out.txt"))
	if err != nil {
		s.Fatal("Failed to create output file: ", err)
	}
	defer f.Close()
	cmd := a.Command(ctx, "am", "instrument", "-w", "org.chromium.arc.file_system.tests/androidx.test.runner.AndroidJUnitRunner")
	cmd.Stdout = f
	cmd.Stderr = f
	if err := cmd.Run(); err != nil {
		s.Error("am instrument failed (see out.txt): ", err)
	}
}
