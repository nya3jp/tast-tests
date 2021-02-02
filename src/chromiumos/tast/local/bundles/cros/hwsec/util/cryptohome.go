// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"context"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/errors"
)

// RequireMountFail would try to mount a user vault, and return error when it success.
func RequireMountFail(ctx context.Context, utility *hwsec.UtilityCryptohomeBinary, user, pass, label string) error {
	err := utility.MountVault(ctx, user, pass, label, true, hwsec.NewVaultConfig())
	if err == nil {
		return errors.Errorf("mount unexpectedly succeeded for %s", user)
	}
	return nil
}
