// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"fmt"
	gotesting "testing"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

func getTests() []*testing.Test {
	const pattern = "arc.*"
	tests, err := testing.GlobalRegistry().TestsForPatterns([]string{pattern})
	if err != nil {
		panic(fmt.Sprintf("Failed to get ARC tests: %v", err))
	}
	if len(tests) == 0 {
		panic(fmt.Sprintf("No tests matched for %s", pattern))
	}
	return tests
}

func TestTimeout(t *gotesting.T) {
	const (
		chromeBootTime  = 60 * time.Second
		minTestBodyTime = 30 * time.Second

		minTimeout = chromeBootTime + arc.BootTimeout + minTestBodyTime
	)

	for _, tst := range getTests() {
		if tst.Timeout < minTimeout {
			t.Errorf("%s: timeout is too short (%v < %v)", tst.Name, tst.Timeout, minTimeout)
		}
	}
}

func TestSoftwareDeps(t *gotesting.T) {
	requiredDeps := []string{"chrome_login", "android"}

	for _, tst := range getTests() {
		deps := make(map[string]struct{})
		for _, d := range tst.SoftwareDeps {
			deps[d] = struct{}{}
		}
		for _, d := range requiredDeps {
			if _, ok := deps[d]; !ok {
				t.Errorf("%s: missing software dependency %q", tst.Name, d)
			}
		}
	}
}
