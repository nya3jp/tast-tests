// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/arc/authperf"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AuthPerfManaged,
		Desc:         "Measure auth times in ARC++ for managed case",
		Contacts:     []string{"niwa@chromium.org", "khmel@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"android", "chrome"},
		Timeout:      20 * time.Minute,
	})
}

func AuthPerfManaged(ctx context.Context, s *testing.State) {
	const (
		username = "autotest-arc-enabled@cros1.managedchrome.com"
		password = "Z=8buZ6T"
	)

	authperf.RunTest(ctx, s, username, password, "_managed")
}
