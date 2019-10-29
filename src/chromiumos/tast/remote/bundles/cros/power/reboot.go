// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"

	"chromiumos/tast/dut"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Reboot,
		Desc:         "Verifies that system comes back after rebooting (with a new reboot method)",
		Contacts:     []string{"nya@chromium.org", "tast-owners@google.com"},
		SoftwareDeps: []string{"reboot"},
		Attr:         []string{"group:mainline"},
	})
}

func Reboot(ctx context.Context, s *testing.State) {
	d, ok := dut.FromContext(ctx)
	if !ok {
		s.Fatal("Failed to get DUT")
	}

	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}
}
