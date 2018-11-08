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

const pattern = "arc.*"

func TestTimeout(t *gotesting.T) {
	const (
		chromeBootTime  = 60 * time.Second
		minTestBodyTime = 30 * time.Second

		minTimeout = chromeBootTime + arc.BootTimeout + minTestBodyTime
	)

	testing.CheckTimeout(t, pattern, minTimeout)
}

func TestSoftwareDeps(t *gotesting.T) {
	testing.CheckSoftwareDeps(t, pattern, []string{"chrome_login", "android"})
}
