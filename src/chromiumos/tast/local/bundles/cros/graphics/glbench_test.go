// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

// To update test parameters after modifying this file, run:
// TAST_GENERATE_UPDATE=1 ~/trunk/src/platform/tast/tools/go.sh test -count=1 chromiumos/tast/local/bundles/cros/graphics/

// See src/chromiumos/tast/local/crostini/params.go for more documentation

import (
	"testing"
	"time"

	"chromiumos/tast/common/genparams"
	"chromiumos/tast/local/crostini"
)

func TestGlBenchParams(t *testing.T) {
	// This test suite has some non-crostini test parameters, so add them here.
	params := `{
			Name:      "",
			Val:       glbenchConfig{environment: envCros},
			Timeout:   3 * time.Hour,
			ExtraAttr: []string{"group:graphics", "graphics_nightly"},
		}, {
			Name:      "hasty",
			Val:       glbenchConfig{hasty: true, environment: envCros},
			ExtraAttr: []string{"group:mainline"},
			Timeout:   5 * time.Minute,
		},`
	params += crostini.MakeTestParamsFromList(t, []crostini.Param{
		{
			Name:              "crostini",
			Timeout:           60 * time.Minute,
			Val:               `glbenchConfig{environment: envDebian}`,
			ExtraSoftwareDeps: []string{"chrome", "crosvm_gpu", "vm_host"},
			ExtraAttr:         []string{"group:graphics", "graphics_weekly"},
			MinimalSet:        true,
			IsNotMainline:     true,
		}, {
			Name:              "crostini_hasty",
			Timeout:           5 * time.Minute,
			Val:               `glbenchConfig{hasty: true, environment: envDebian}`,
			ExtraSoftwareDeps: []string{"chrome", "crosvm_gpu", "vm_host"},
			ExtraAttr:         []string{"group:graphics", "graphics_perbuild", "group:mainline", "informational"},
			MinimalSet:        true,
		}})

	genparams.Ensure(t, "glbench.go", params)
}
