// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"testing"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing/testcheck"
)

const pattern = "arc.*"

func TestTimeout(t *testing.T) {
	const (
		chromeBootTime  = 60 * time.Second
		minTestBodyTime = 30 * time.Second

		minTimeout = chromeBootTime + arc.BootTimeout + minTestBodyTime
	)

	testcheck.Timeout(t, pattern, minTimeout)
}

func TestSoftwareDeps(t *testing.T) {
	testcheck.SoftwareDeps(t, pattern, []string{"chrome_login", "android|android_p|android_all"})
}
