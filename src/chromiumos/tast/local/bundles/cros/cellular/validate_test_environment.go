// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"chromiumos/tast/local/cellular"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ValidateTestEnvironment,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies that host has signal quality above threshold via cellular interface",
		Contacts:     []string{"nmarupaka@google.com", "chromeos-cellular-team@google.com"},
		Attr:         []string{"group:cellular", "cellular_unstable", "cellular_sim_active"},
		Fixture:      "cellularConnected",
		Timeout:      3 * time.Minute,
		SoftwareDeps: []string{"chrome"},
	})
}

// ValidateTestEnvironment needs to be run to verify that the DUT has sufficient signal coverage to execute other network related test cases
func ValidateTestEnvironment(ctx context.Context, s *testing.State) {
	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}
}
