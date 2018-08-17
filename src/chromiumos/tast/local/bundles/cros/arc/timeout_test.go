// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	gotesting "testing"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

func TestTimeout(t *gotesting.T) {
	const (
		chromeBootTime  = 60 * time.Second
		minTestBodyTime = 30 * time.Second

		minTimeout = chromeBootTime + arc.BootTimeout + minTestBodyTime
	)

	tests, err := testing.GlobalRegistry().TestsForPatterns([]string{"arc.*"})
	if err != nil {
		t.Fatal("Failed to get ARC tests: ", err)
	}
	if len(tests) == 0 {
		t.Fatal("No tests matched for arc.*")
	}

	for _, tst := range tests {
		if tst.Timeout < minTimeout {
			t.Errorf("%s: timeout is too short (%v < %v)", tst.Name, tst.Timeout, minTimeout)
		}
	}
}
