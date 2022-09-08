// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typec

import (
	"context"

	"chromiumos/tast/local/typecutils"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Typecd,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Checks USB Type C mode switch behaviour when typecd crashes",
		Contacts:     []string{"pmalani@chromium.org", "chromeos-power@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		HardwareDeps: hwdep.D(hwdep.ECFeatureTypecCmd(), hwdep.ChromeEC()),
	})
}

// Typecd checks that typecd is running on the system.
func Typecd(ctx context.Context, s *testing.State) {
	_, err := typecutils.TypecdPID(ctx)
	if err != nil {
		s.Fatal("Failed to get typecd PID: ", err)
	}
}
