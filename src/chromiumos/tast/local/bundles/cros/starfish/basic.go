// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package starfish

import (
	"context"
	"time"

	"chromiumos/tast/local/starfish"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Basic,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies that host has network connectivity via cellular interface",
		Contacts:     []string{"nmarupaka@google.com", "chromeos-cellular-team@google.com"},
		Attr:         []string{"group:cellular", "cellular_unstable", "cellular_sim_active"},
		Timeout:      1 * time.Minute,
		SoftwareDeps: []string{"chrome"},
	})
}

// Basic tests if the helper is being created successfully.
func Basic(ctx context.Context, s *testing.State) {
	_, err := starfish.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create starfish.Helper: ", err)
	}
}
