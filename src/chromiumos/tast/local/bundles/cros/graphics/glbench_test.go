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
			Val:       config{config: &glbench.CrosConfig{}},
			Timeout:   3 * time.Hour,
			ExtraAttr: []string{"group:graphics", "graphics_nightly"},
			Fixture:   "graphicsNoChrome",
		}, {
			Name:      "hasty",
			Val:       config{config: &glbench.CrosConfig{Hasty: true}},
			ExtraAttr: []string{"group:mainline"},
			Timeout:   5 * time.Minute,
			Fixture:   "graphicsNoChrome",
		},`
	params += crostini.MakeTestParamsFromList(t, []crostini.Param{
		{
			Name:              "crostini",
			Timeout:           60 * time.Minute,
			Val:               `config{config: &glbench.CrostiniConfig{}}`,
			ExtraSoftwareDeps: []string{"chrome", "crosvm_gpu", "vm_host"},
			ExtraAttr:         []string{"group:graphics", "graphics_nightly"},
			MinimalSet:        true,
			IsNotMainline:     true,
		}, {
			Name:              "crostini_hasty",
			Timeout:           5 * time.Minute,
			Val:               `config{config: &glbench.CrostiniConfig{Hasty: true}}`,
			ExtraSoftwareDeps: []string{"chrome", "crosvm_gpu", "vm_host"},
			ExtraAttr:         []string{"group:graphics", "graphics_perbuild", "group:mainline", "informational"},
			MinimalSet:        true,
		}})

	genparams.Ensure(t, "glbench.go", params)
}
