// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package subtest

import (
	"context"

	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// UninstallApplication tests uninstalling the application installed by the
// install package test.
func UninstallApplication(ctx context.Context, s *testing.State,
	cont *vm.Container, desktopFileID string) {
	testing.ContextLog(ctx, "Executing UninstallPackageOwningFile test")
	err := cont.UninstallPackageOwningFile(ctx, desktopFileID)
	if err != nil {
		s.Error("Failed executing UninstallPackageOwningFile: ", err)
		return
	}
}
