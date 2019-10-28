// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"os"
	"path/filepath"

	"chromiumos/tast/crash"
	"chromiumos/tast/local/arc"
	localCrash "chromiumos/tast/local/crash"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AppCrash,
		Desc:         "Test handling of a local app crash",
		Contacts:     []string{"mutexlox@google.com", "cros-monitoring-forensics@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android", "chrome", "chrome_internal"},
		Pre:          arc.Booted(),
	})
}

func AppCrash(ctx context.Context, s *testing.State) {
	const (
		pkg = "com.android.settings"
		cls = ".Settings"
	)
	if err := localCrash.SetUpCrashTest(); err != nil {
		s.Fatal("Couldn't set up crash test: ", err)
	}
	defer localCrash.TearDownCrashTest()

	a := s.PreValue().(arc.PreData).ARC
	cr := s.PreValue().(arc.PreData).Chrome

	act, err := arc.NewActivity(a, pkg, cls)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	s.Log("Starting app")
	if err = act.Start(ctx); err != nil {
		s.Fatal("Failed to start app: ", err)
	}

	s.Log("Getting preexisting crashes")
	user := cr.User()
	path, err := cryptohome.UserPath(ctx, user)
	if err != nil {
		s.Fatal("Couldn't get user path: ", err)
	}
	crashDir := filepath.Join(path, "/crash")

	oldCrashes, err := crash.GetCrashes(crashDir)
	if err != nil {
		s.Fatal("Couldn't get preexisting crashes: ", err)
	}

	if err := a.Command(ctx, "am", "crash", pkg).Run(testexec.DumpLogOnError); err != nil {
		s.Fatalf("Couldn't kill app: %s", err)
	}

	s.Log("Waiting for crash files to become present")
	files, err := localCrash.WaitForCrashFiles(ctx, []string{crashDir}, oldCrashes, []string{
		`com_android_settings.\d{8}.\d{6}.\d+.log`,
		`com_android_settings.\d{8}.\d{6}.\d+.meta`,
	})
	if err != nil {
		s.Error("didn't find files: ", err)
	}

	for _, f := range files {
		if err := os.Remove(f); err != nil {
			s.Errorf("Couldn't clean up %s: %v", f, err)
		}
	}
}
