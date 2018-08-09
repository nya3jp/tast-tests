// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arcapp

import (
	"testing"
	"time"

	"chromiumos/tast/local/arc"
	tasttesting "chromiumos/tast/testing"
)

func TestTimeout(t *testing.T) {
	const (
		chromeBootTime  = 60 * time.Second
		minTestBodyTime = 30 * time.Second

		minTimeout = chromeBootTime + arc.BootTimeout + minTestBodyTime
	)

	tests, err := tasttesting.GlobalRegistry().TestsForPatterns([]string{"arcapp.*"})
	if err != nil {
		t.Fatal("Failed to get ARC app tests: ", err)
	}
	if len(tests) == 0 {
		t.Fatal("No tests matched for arcapp.*")
	}

	for _, tst := range tests {
		if tst.Timeout < minTimeout {
			t.Errorf("%s: timeout is too short (%v < %v)", tst.Name, tst.Timeout, minTimeout)
		}
	}
}
