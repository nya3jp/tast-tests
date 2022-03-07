// Copyright 2020 The Chromium OS Authors. All rights reserved.
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
)

func TestDisplayDensityParams(t *testing.T) {
	params := crostini.MakeTestParamsFromList(t, []crostini.Param{
		{
			Name:       "wayland",
			Val:        "crostini.WaylandDemoConfig()",
			UseFixture: true,
		},
		{
			Name:       "x11",
			Val:        "crostini.X11DemoConfig()",
			UseFixture: true,
		}})
	genparams.Ensure(t, "display_density.go", params)
}
