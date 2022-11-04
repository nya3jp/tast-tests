// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

// To update test parameters after modifying this file, run:
// TAST_GENERATE_UPDATE=1 ~/trunk/src/platform/tast/tools/go.sh test -count=1 chromiumos/tast/local/bundles/cros/network/

// See src/chromiumos/tast/local/crostini/params.go for more documentation

import (
	"testing"
	"time"

	"chromiumos/tast/common/genparams"
	"chromiumos/tast/local/crostini"
)

func TestCrostiniConnectivityParams(t *testing.T) {
	params := crostini.MakeTestParamsFromList(t, []crostini.Param{{
		Timeout:       10 * time.Minute,
		IsNotMainline: true,
		UseFixture:    true,
	}})
	genparams.Ensure(t, "crostini_connectivity.go", params)
}
