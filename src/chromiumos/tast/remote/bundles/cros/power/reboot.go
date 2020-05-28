// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Reboot,
		Desc:         "Verifies that system comes back after rebooting",
		Contacts:     []string{"tast-owners@google.com"},
		SoftwareDeps: []string{"reboot"},
		Attr:         []string{"group:mainline", "group:labqual"},
	})
}

func Reboot(ctx context.Context, s *testing.State) {
	d := s.DUT()

	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}
}
