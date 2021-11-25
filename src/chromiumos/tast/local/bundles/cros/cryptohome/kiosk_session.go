// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/local/cryptohome"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: KioskSession,
		Desc: "Ensures that cryptohome correctly mounts kiosk sessions",
		Contacts: []string{
			"hardikgoyal@chromium.org",
			"chromeos-security@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

func KioskSession(ctx context.Context, s *testing.State) {
	cmdRunner := hwseclocal.NewLoglessCmdRunner()
	cryptohomeClient := hwsec.NewCryptohomeClient(cmdRunner)
	mountInfo := hwsec.NewCryptohomeMountInfo(cmdRunner, cryptohomeClient)

	// Unmount all user vaults before we start.
	if err := cryptohomeClient.UnmountAll(ctx); err != nil {
		s.Log("Failed to unmount all before test starts: ", err)
	}

	if err := cryptohomeClient.MountKiosk(ctx); err != nil {
		s.Fatal("Failed to mount kiosk: ", err)
	}
	// Unmount Vault.
	cryptohomeClient.Unmount(ctx, hwsec.KioskUser)

	// Test if existing kiosk vault can be signed in with AuthSession.
	if err := cryptohome.AuthSessionMountFlow(ctx, cryptohomeClient, mountInfo, true /*Kiosk User*/, hwsec.KioskUser, "" /* Empty passkey*/, false /*Create User*/); err != nil {
		s.Fatal("Failed to Mount with AuthSession -: ", err)
	}
}
