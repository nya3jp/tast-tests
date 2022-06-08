// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

// To update test parameters after modifying this file, run:
// TAST_GENERATE_UPDATE=1 ~/trunk/src/platform/tast/tools/go.sh test -count=1 chromiumos/tast/local/bundles/cros/crostini/

// See src/chromiumos/tast/local/crostini/params.go for more documentation

import (
	"testing"

	"chromiumos/tast/common/genparams"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/crostini/ui"
)

func TestCheckValidUsersParams(t *testing.T) {
	params := crostini.MakeTestParamsFromList(t, []crostini.Param{{
		// validUsersMap has 4 user categories. In the test we install Crostini for every category.
		Timeout:            4 * ui.InstallationTimeout,
		MinimalSet:         true,
		SelfManagedInstall: true,
	}})
	genparams.Ensure(t, "check_valid_users.go", params)
}
