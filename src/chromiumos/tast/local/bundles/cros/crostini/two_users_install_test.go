// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"testing"
	"time"

	"chromiumos/tast/common/genparams"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/vm"
)

func TestTwoUsersInstallParams(t *testing.T) {
	params := crostini.MakeTestParamsFromList(t, []crostini.Param{{
		Timeout: 14 * time.Minute,
		Preconditions: map[vm.ContainerDebianVersion]string{
			vm.DebianBuster: "nil",
		},
		MinimalSet: true,
	}})
	genparams.Ensure(t, "two_users_install.go", params)
}
