// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

// To update test parameters after modifying this file, run:
// TAST_GENERATE_UPDATE=1 ~/trunk/src/platform/tast/tools/go.sh test -count=1 chromiumos/tast/local/bundles/cros/crostini/

// See src/chromiumos/tast/local/crostini/params.go for more documentation

import (
	"testing"
	"time"

	"chromiumos/tast/common/genparams"
	"chromiumos/tast/local/crostini"
)

func TestCheckValidUsersParams(t *testing.T) {
	params := crostini.MakeTestParamsFromList(t, []crostini.Param{{
		Timeout:            20 * time.Minute,
		MinimalSet:         true,
		SelfManagedInstall: true,
	}})
	genparams.Ensure(t, "check_valid_users.go", params)
}
