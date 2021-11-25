// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"

	"chromiumos/tast/common/hwsec"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: GuestSession,
		Desc: "Ensures that cryptohome correctly mounts guest sessions",
		Contacts: []string{
			"jorgelo@chromium.org",
			"chromeos-security@google.com",
		},
		Attr: []string{"group:mainline"},
	})
}

func GuestSession(ctx context.Context, s *testing.State) {
	// Guest session is mounted in the user session mount namespace, so first
	// check whether the namespace is created.
	cmdRunner := hwseclocal.NewLoglessCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)
	mountInfo := hwsec.NewCryptohomeMountInfo(cmdRunner, cryptohome)
	if err := mountInfo.CheckMountNamespace(ctx); err != nil {
		s.Log("Mount namespace is not ready: ", err)
	}
	if err := cryptohome.MountGuest(ctx); err != nil {
		s.Fatal("Failed to mount guest: ", err)
	}
	defer cryptohome.Unmount(ctx, hwsec.GuestUser)
}
