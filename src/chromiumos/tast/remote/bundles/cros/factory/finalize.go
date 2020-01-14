// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package factory

import (
	"context"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Finalize,
		Desc:     "Test finalize process in factory toolkit",
		Contacts: []string{"menghuan@chromium.org", "chromeos-factory-eng@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

func Finalize(ctx context.Context, s *testing.State) {
	d := s.DUT()

	// "gooftool" of "factory-mini" has already been installed on test image.
	if err := d.Command("gooftool", "wipe_in_place", "--test_umount").Run(ctx); err != nil {
		s.Fatal("Failed to run wiping of finalize: ", err)
	}

	// Reboot to recover umounted partitiions.
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}
}
