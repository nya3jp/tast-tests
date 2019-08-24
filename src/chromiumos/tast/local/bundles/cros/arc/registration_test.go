// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	gotesting "testing"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/testcheck"
)

const pattern = "arc.*"

func TestTimeout(t *gotesting.T) {
	const minTestBodyTime = 30 * time.Second
	minTimeout := chrome.LoginTimeout + arc.BootTimeout + minTestBodyTime

	re, err := testing.NewTestGlobRegexp(pattern)
	if err != nil {
		t.Fatalf("Bad glob %q: %v", pattern, err)
	}
	filter := func(t *testing.TestCase) bool {
		// Only arc.* tests are interesting.
		if !re.MatchString(t.Name) {
			return false
		}
		// If the test has arc.Booted() precondition, it is not necessary to extend
		// the timeout, so skip them.
		if t.Pre == arc.Booted() {
			return false
		}
		return true
	}
	testcheck.Timeout(t, filter, minTimeout)
}

func TestSoftwareDeps(t *gotesting.T) {
	testcheck.SoftwareDeps(t, testcheck.Glob(t, pattern), []string{"chrome", "android|android_vm|android_both|android_p|android_p_both|android_all|android_all_both"})
}
