// Copyright 2020 The ChromiumOS Authors
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

func TestToolkitParams(t *testing.T) {
	params := crostini.MakeTestParamsFromList(t, []crostini.Param{
		{
			Name:      "gtk3_wayland",
			ExtraData: []string{"toolkit_gtk3_demo.py"},
			Val: `toolkitConfig{
				data:    "toolkit_gtk3_demo.py",
				command: []string{"env", "GDK_BACKEND=wayland", "python3", "toolkit_gtk3_demo.py"},
				appID:   "crostini:toolkit_gtk3_demo.py",
			}`,
			UseFixture: true,
		}, {
			Name:      "gtk3_x11",
			ExtraData: []string{"toolkit_gtk3_demo.py"},
			Val: `toolkitConfig{
				data:    "toolkit_gtk3_demo.py",
				command: []string{"env", "GDK_BACKEND=x11", "python3", "toolkit_gtk3_demo.py"},
				appID:   "crostini:org.chromium.termina.wmclass.Toolkit_gtk3_demo.py",
			}`,
			UseFixture: true,
		}, {
			Name:      "qt5",
			ExtraData: []string{"toolkit_qt5_demo.py"},
			Val: `toolkitConfig{
				data:    "toolkit_qt5_demo.py",
				command: []string{"python3", "toolkit_qt5_demo.py"},
				appID:   "crostini:org.chromium.termina.wmclass.toolkit_qt5_demo.py",
			}`,
			UseFixture: true,
		}, {
			Name:      "tkinter",
			ExtraData: []string{"toolkit_tkinter_demo.py"},
			Val: `toolkitConfig{
				data:    "toolkit_tkinter_demo.py",
				command: []string{"python3", "toolkit_tkinter_demo.py"},
				appID:   "crostini:org.chromium.termina.wmclass.Tkinter_demo",
			}`,
			UseFixture: true,
		}})
	genparams.Ensure(t, "toolkit.go", params)
}
