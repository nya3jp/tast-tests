// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"time"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     RemoteReboot,
		Desc:     "Reboot a DUT to the tast streaming logs correctly",
		Contacts: []string{"tast-owners@google.com"},
	})
}

func RemoteReboot(ctx context.Context, s *testing.State) {
	d := s.DUT()
	s.Log("Rebooting DUT")
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}
	s.Log("DUT is up now")
	// This sleep is for testing log streaming.
	testing.Sleep(ctx, time.Minute*5)
}
