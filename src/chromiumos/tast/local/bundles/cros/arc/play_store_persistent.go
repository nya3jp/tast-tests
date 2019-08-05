// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PlayStorePersistent,
		Desc:         "Makes sure that Play Store is persistent in tests",
		Contacts:     []string{"khmel@chromium.org", "jhorwich@chromium.org", "arc-core@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android_all_both", "chrome"},
		Timeout:      5 * time.Minute,
	})
}

// getPlayStorePid scans acive processes and finds process com.android.vending. Found process is
// returned as PID. In case com.android.vending is not found error is returned.
func getPlayStorePid(ctx context.Context, a *arc.ARC) (uint, error) {
	out, err := a.Command(ctx, "ps", "-A", "-o", "PID", "-o", "NAME").CombinedOutput()
	if err != nil {
		return 0, err
	}

	m := regexp.MustCompile(`\ *(\d+) com\.android\.vending\n`).FindAllStringSubmatch(string(out), -1)
	if m == nil || len(m) != 1 {
		return 0, errors.New("could not find Play Store app")
	}

	pid, err := strconv.ParseUint(m[0][1], 10, 32)
	if err != nil {
		return 0, err
	}

	return uint(pid), nil
}

func PlayStorePersistent(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ARCEnabled(),
		chrome.ExtraArgs("--arc-disable-app-sync", "--arc-disable-play-auto-install", "--arc-disable-locale-sync", "--arc-play-store-auto-update=off"))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	pidBefore, err := getPlayStorePid(ctx, a)
	if err != nil {
		s.Fatal("Failed to get initial PlayStore PID: ", err)
	}

	s.Log("Waiting")
	if err := testing.Sleep(ctx, 3*time.Minute); err != nil {
		s.Fatal("Timed out while sleeping: ", err)
	}

	pidAfter, err := getPlayStorePid(ctx, a)
	if err != nil {
		s.Fatal("Failed to get PlayStore PID: ", err)
	}

	if pidAfter != pidBefore {
		s.Fatal("Play Store was restarted")
	}
}
