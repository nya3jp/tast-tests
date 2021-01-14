// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ShillCellularReady,
		Desc: "Verifies that Shill is running and that a Cellular Device and connectable Service are present",
		Contacts: []string{
			"stevenjb@google.com",
			"cros-network-health@google.com",
			"chromeos-cellular-team@google.com",
		},
		Attr: []string{"group:cellular"},
	})
}

// ShillCellularReady Test. Note: This test enables Cellular if not already enabled.
func ShillCellularReady(ctx context.Context, s *testing.State) {
	helper, err := NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create Helper: ", err)
	}
	// Ensure that a Cellular Service was created.
	if _, err := helper.FindService(ctx); err != nil {
		s.Fatal("Unable to find Cellular Service: ", err)
	}
}
