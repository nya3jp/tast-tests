// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package autoupdate

import (
	"context"

	"chromiumos/tast/remote/bundles/cros/autoupdate/compat"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         NToMVaultCompatEcryptfs,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify cross version vault's compatibility",
		Contacts: []string{
			"dlunev@google.com", // Test author
			"chromeos-storage@google.com",
		},
		Attr:         []string{}, // Manual execution only.
		SoftwareDeps: []string{"tpm", "reboot", "chrome"},
		ServiceDeps: []string{
			"tast.cros.autoupdate.NebraskaService",
			"tast.cros.autoupdate.UpdateService",
		},
		Timeout: compat.TotalTestTime,
	})
}

func NToMVaultCompatEcryptfs(ctx context.Context, s *testing.State) {
	compat.NToMVaultCompatImpl(ctx, s, compat.EcryptfsVaultType)
}
