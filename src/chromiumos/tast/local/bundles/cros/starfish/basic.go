// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package starfish

import (
	"context"
	"time"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Basic,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies basic Starfish functionality",
		Contacts:     []string{"nmarupaka@google.com", "chromeos-cellular-team@google.com"},
		Attr:         []string{"group:cellular", "cellular_starfish"},
		Fixture:      "starfish",
		Timeout:      1 * time.Minute,
		SoftwareDeps: []string{"chrome"},
	})
}

// Basic test to verify if the Starfish module is being initialized successfully.
func Basic(ctx context.Context, s *testing.State) {
	testing.ContextLog(ctx, "Waiting 5 seconds")
	testing.Sleep(ctx, 2*time.Second)
	testing.ContextLog(ctx, "Done")
}
