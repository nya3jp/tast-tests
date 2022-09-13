// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hypervisor

import (
	"context"

	"chromiumos/tast/remote/hypervisor"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         IsManatee,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify that manatee detection is accurate",
		Contacts:     []string{"psoberoi@google.com", "manateam@google.com"},
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			Name:              "without_manatee",
			ExtraSoftwareDeps: []string{"no_manatee"},
			Val:               false,
		}, {
			Name:              "with_manatee",
			ExtraSoftwareDeps: []string{"manatee"},
			Val:               true,
		}},
	})
}

func IsManatee(ctx context.Context, s *testing.State) {
	manateeExpected := s.Param().(bool)

	d := s.DUT()
	manatee, err := hypervisor.IsManatee(ctx, d)
	if err != nil {
		s.Fatal("WARNING: Failed to check for ManaTEE: ", err)
	}
	if manatee != manateeExpected {
		s.Errorf("Unexpected result from IsManatee: %v (expected %v)", manatee, manateeExpected)
	}
}
