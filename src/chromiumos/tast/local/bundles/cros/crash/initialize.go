// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"io/ioutil"
	"strings"

	commoncrash "chromiumos/tast/common/crash"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Initialize,
		Desc:         "Verifies that the crash reporter initializes core_pattern, even without metrics consent",
		Contacts:     []string{"mutexlox@google.com", "cros-telemetry@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "metrics_consent"},
		Pre:          crash.ChromePreWithVerboseConsent(),
	})
}

func Initialize(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	if err := crash.SetUpCrashTest(ctx, crash.FilterCrashes(crash.FilterInIgnoreAllCrashes)); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer crash.TearDownCrashTest(ctx)

	// Disable consent.
	if err := crash.SetConsent(ctx, cr, false); err != nil {
		s.Fatal("SetConsent failed: ", err)
	}

	success := false
	defer func() {
		// Restore expected contents if crash_reporter --init didn't work,
		// so that no matter what this test will end with the expected core pattern.
		if !success {
			if err := ioutil.WriteFile(commoncrash.CorePattern, []byte(commoncrash.ExpectedCorePattern()), 0644); err != nil {
				s.Errorf("Failed restoring core pattern file %s: %s", commoncrash.CorePattern, err)
			}
		}
	}()

	// Set core pattern to something invalid.
	if err := ioutil.WriteFile(commoncrash.CorePattern, []byte("core"), 0644); err != nil {
		s.Fatalf("Failed writing core pattern file %s: %s",
			commoncrash.CorePattern, err)
	}

	// initialize crash_reporter
	if err := testexec.CommandContext(ctx, "/sbin/crash_reporter", "--init").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Could not run crash_reporter: ", err)
	}

	// verify contents.
	b, err := ioutil.ReadFile(commoncrash.CorePattern)
	if err != nil {
		s.Fatalf("Failed reading core pattern file %s: %s",
			commoncrash.CorePattern, err)
	}
	trimmed := strings.TrimSuffix(string(b), "\n")
	if expected := commoncrash.ExpectedCorePattern(); trimmed != expected {
		s.Fatalf("Unexpected core_pattern: got %s, want %s", trimmed, expected)
	}
	success = true
}
