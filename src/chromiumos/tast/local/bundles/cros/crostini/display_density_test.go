// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"testing"

	"chromiumos/tast/common/genparams"
	"chromiumos/tast/local/crostini"
)

func TestDisplayDensityParams(t *testing.T) {
	params := crostini.MakeTestParamsFromList(t, []crostini.Param{
		{
			Name: "wayland",
			Val:  "crostini.WaylandDemoConfig()",
		},
		{
			Name: "x11",
			Val:  "crostini.X11DemoConfig()",
		}})
	genparams.Ensure(t, "display_density.go", params)
}
