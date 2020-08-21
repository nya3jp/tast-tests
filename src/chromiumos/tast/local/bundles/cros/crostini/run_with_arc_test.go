// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"testing"

	"chromiumos/tast/common/genparams"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/vm"
)

func TestRunWithARCParams(t *testing.T) {
	params := crostini.MakeTestParamsFromList(t, []crostini.Param{{
		Preconditions: map[vm.ContainerDebianVersion]string{
			vm.DebianBuster: "crostini.StartedARCEnabled()",
		},
		MinimalSet: true,
	}})
	genparams.Ensure(t, "run_with_arc.go", params)
}
