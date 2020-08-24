// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"testing"

	"chromiumos/tast/common/genparams"
	"chromiumos/tast/local/crostini"
)

func TestGpuEnabledParams(t *testing.T) {
	params := crostini.MakeTestParamsFromList(t, []crostini.Param{
		{
			Name:              "sw",
			Val:               `"llvmpipe"`,
			ExtraSoftwareDeps: []string{"crosvm_no_gpu"},
		},
		{
			Name:              "gpu",
			Val:               `"virgl"`,
			ExtraSoftwareDeps: []string{"crosvm_gpu"},
		}})
	genparams.Ensure(t, "gpu_enabled.go", params)
}
