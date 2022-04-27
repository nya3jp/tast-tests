// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lacros tests lacros-chrome running on ChromeOS.
package lacros

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/chrome/lacros/lacrosfaillog"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/set"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Basic,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests basic lacros startup",
		Contacts:     []string{"erikchen@chromium.org", "hidehiko@chromium.org", "edcourtney@chromium.org", "lacros-team@google.com", "chromeos-sw-engprod@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Fixture:      "lacros",
		Timeout:      7 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"lacros_stable"},
		}, {
			Name:              "unstable",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"lacros_unstable"},
		}},
	})
}

func crashFiles() ([]string, error) {
	var result []string
	for _, dir := range crash.DefaultDirs() {
		crashFiles, err := crash.GetCrashes(dir)
		if err != nil {
			return nil, err
		}
		result = append(result, crashFiles...)
	}
	return result, nil
}

func Basic(ctx context.Context, s *testing.State) {
	// Fail if crashes are reported during the test.
	crashFilesBefore, err := crashFiles()
	if err != nil {
		s.Fatal("Failed to read crash directory: ", err)
	}
	defer func() {
		// Wait a few seconds for any reports to get written to disk.
		if err := testing.Sleep(ctx, 5*time.Second); err != nil {
			s.Fatal("Failed to sleep: ", err)
		}
		crashFilesAfter, err := crashFiles()
		if err != nil {
			s.Fatal("Failed to read crash directory: ", err)
		}
		newCrashFiles := set.DiffStringSlice(crashFilesAfter, crashFilesBefore)
		if len(newCrashFiles) != 0 {
			s.Fatal("Detected new crash reports (see the \"crashes\" directory in the Tast results):\n\t" + strings.Join(newCrashFiles, "\n\t"))
		}
	}()

	tconn, err := s.FixtValue().(chrome.HasChrome).Chrome().TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	l, err := lacros.Launch(ctx, tconn)
	defer lacrosfaillog.SaveIf(ctx, tconn, s.HasError)
	if err != nil {
		s.Fatal("Failed to launch lacros-chrome: ", err)
	}
	defer l.Close(ctx)

	// Test that a new blank tab can be opened.
	conn, err := l.NewConn(ctx, "about:blank")
	if err != nil {
		s.Fatal("Failed to open new tab: ", err)
	}
	if err := conn.Close(); err != nil {
		s.Error("Failed to close connection: ", err)
	}
}
