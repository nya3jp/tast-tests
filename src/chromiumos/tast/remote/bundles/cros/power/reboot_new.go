// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RebootNew,
		Desc:         "Verifies that system comes back after rebooting (with a new reboot method)",
		Contacts:     []string{"nya@chromium.org", "tast-owners@google.com"},
		SoftwareDeps: []string{"reboot"},
		// TODO(crbug.com/1000505): Replace power.Reboot with this test once making sure it is stable.
		Attr: []string{"group:mainline"},
	})
}

func RebootNew(ctx context.Context, s *testing.State) {
	d := s.DUT()
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}
}
