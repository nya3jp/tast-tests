// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/local/chrome"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LoginGuest,
		Desc: "Verifies the cryptohome is mounted for guest user login",
		Contacts: []string{
			"achuith@chromium.org",  // Original autotest author
			"hidehiko@chromium.org", // Tast port author
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "group:labqual"},
	})
}

func LoginGuest(ctx context.Context, s *testing.State) {
	cmdRunner := hwseclocal.NewLoglessCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)
	mountInfo := hwsec.NewCryptohomeMountInfo(cmdRunner, cryptohome)

	func() {
		cr, err := chrome.New(ctx, chrome.GuestLogin())
		if err != nil {
			s.Fatal("Failed to log in by Chrome: ", err)
		}
		defer cr.Close(ctx)

		if mounted, err := mountInfo.IsMounted(ctx, hwsec.GuestUser); err != nil {
			s.Error("Failed to check mounted vault for guest user: ", err)
		} else if !mounted {
			s.Error("No mounted vault for guest user")
		}
	}()

	// Emulate logout. chrome.Chrome.Close() does not log out. So, here,
	// manually restart "ui" job for the emulation.
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to log out: ", err)
	}

	if mounted, err := mountInfo.IsMounted(ctx, hwsec.GuestUser); err != nil {
		s.Error("Failed to check mounted vault for guest user: ", err)
	} else if mounted {
		s.Error("Mounted vault for guest user is still found after logout")
	}
}
