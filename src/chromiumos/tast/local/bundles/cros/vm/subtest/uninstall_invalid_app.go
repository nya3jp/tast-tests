// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package subtest

import (
	"context"
	"strings"

	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// UninstallInvalidApplication attempts to uninstall a non-existant desktop
// file. Expects to see errors.
func UninstallInvalidApplication(ctx context.Context, s *testing.State,
	cont *vm.Container) {
	testing.ContextLog(ctx, "Executing bad UninstallPackageOwningFile test")
	err := cont.UninstallPackageOwningFile(ctx, "bad")
	if err == nil {
		s.Error("Did not fail when attempting invalid UninstallPackageOwningFile")
		return
	}
	if !strings.Contains(err.Error(), "desktop_file_id does not exist") {
		s.Error("Did not get expected error messages when running invalid UninstallPackageOwningFile: ", err)
	}
}
