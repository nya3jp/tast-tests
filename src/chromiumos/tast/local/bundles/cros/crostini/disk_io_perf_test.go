// Copyright 2020 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/vm"
)

func TestDiskIoPerfParams(t *testing.T) {
	params := crostini.MakeTestParamsFromList(t, []crostini.Param{{
		Timeout: 60 * time.Minute,
		Preconditions: map[vm.ContainerDebianVersion]string{
			vm.DebianBuster: "crostini.StartedTraceVM()",
		},
		MinimalSet:    true,
		IsNotMainline: true,
	}})
	genparams.Ensure(t, "disk_io_perf.go", params)
}
