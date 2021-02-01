// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Kmsvnc,
		Desc:     "Make sure kmsvnc doesn't crash",
		Contacts: []string{"uekawa@chromium.org"},
		Attr:     []string{"group:mainline", "informational"},
		// TODO check what screenshot dep is.
		SoftwareDeps: []string{"chrome", "screenshot"},
		// TODO check if we need chrome graphics.
		Fixture: "chromeGraphics",
	})
}

func findExecutable(name string, paths []string) (string, error) {
	for _, path := range paths {
		fullName := filepath.Join(path, name)
		s, err := os.Stat(fullName)
		if os.IsNotExist(err) {
			continue
		}
		if s.Mode().Perm()&0o100 == 0 {
			continue
		}
		log.Printf("%v exists!", fullName)
		return fullName, nil
	}
	return "", errors.Errorf("no match found for %v", name)
}

func Kmsvnc(ctx context.Context, s *testing.State) {
	kmsvncPath, err := findExecutable("kmsvnc", []string{"/usr/sbin/", "/usr/local/sbin/"})
	if err != nil {
		s.Fatal("Cannot find kmsvnc binary: ", err)
	}

	// TODO(uekawa): limit method=egl to devices that do support it.
	for _, method := range []string{
		"--method=bo",
		"--method=egl",
	} {
		// Run kmsvnc for a second and let it dump logs.
		kmsvncCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
		defer cancel()
		cmd := testexec.CommandContext(kmsvncCtx, kmsvncPath, method, "--v=2")
		out, err := cmd.CombinedOutput()
		if err != nil {
			if err != context.DeadlineExceeded {
				s.Fatal("Failure in kmsvnc execution: ", err)
			} else {
				// Exceeding context deadline is desirable, it survived 1 second!
			}
		}
		//TODO: check output and scan for useful information.
		log.Printf("%v\n", out)
	}

	// TODO(uekawa): do I need to re-enable screen wake lock? Why?
	// For cleanup, re-enable screen_wake_lock
	testexec.CommandContext(ctx, "set_power_policy", "--screen_wake_lock=0").Run()
}
