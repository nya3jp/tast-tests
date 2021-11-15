// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	gotesting "testing"

	_ "chromiumos/tast/local/bundles/cros/localtests"
	_ "chromiumos/tast/remote/bundles/cros/remotetests"
	// "chromiumos/tast/testing"
	"chromiumos/tast/testing/testcheck"
	// "strings"
)

func TestFixtTest(t *gotesting.T) {
	testcheck.FixtureDeps(t)
}
