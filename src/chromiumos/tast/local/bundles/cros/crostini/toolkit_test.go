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

func TestToolkitParams(t *testing.T) {
	params := crostini.MakeTestParamsFromList(t, []crostini.Param{
		{
			Name:      "gtk3_wayland",
			ExtraData: []string{"toolkit_gtk3_demo.py"},
			Val: `toolkitConfig{
				data:    "toolkit_gtk3_demo.py",
				deps:    []string{"python3-gi", "python3-gi-cairo", "gir1.2-gtk-3.0"},
				command: []string{"env", "GDK_BACKEND=wayland", "python3", "toolkit_gtk3_demo.py"},
				appID:   "crostini:toolkit_gtk3_demo.py",
			}`,
		}, {
			Name:      "gtk3_x11",
			ExtraData: []string{"toolkit_gtk3_demo.py"},
			Val: `toolkitConfig{
				data:    "toolkit_gtk3_demo.py",
				deps:    []string{"python3-gi", "python3-gi-cairo", "gir1.2-gtk-3.0"},
				command: []string{"env", "GDK_BACKEND=x11", "python3", "toolkit_gtk3_demo.py"},
				appID:   "crostini:org.chromium.termina.wmclass.Toolkit_gtk3_demo.py",
			}`,
		}, {
			Name:      "qt5",
			ExtraData: []string{"toolkit_qt5_demo.py"},
			Val: `toolkitConfig{
				data:    "toolkit_qt5_demo.py",
				deps:    []string{"python3-pyqt5"},
				command: []string{"python3", "toolkit_qt5_demo.py"},
				appID:   "crostini:org.chromium.termina.wmclass.toolkit_qt5_demo.py",
			}`,
		}, {
			Name:      "tkinter",
			ExtraAttr: []string{"informational"}, /* b/200056776 */
			ExtraData: []string{"toolkit_tkinter_demo.py"},
			Val: `toolkitConfig{
				data:    "toolkit_tkinter_demo.py",
				deps:    []string{"python3-tk"},
				command: []string{"python3", "toolkit_tkinter_demo.py"},
				appID:   "crostini:org.chromium.termina.wmclass.Tkinter_demo",
			}`,
		}})
	genparams.Ensure(t, "toolkit.go", params)
}
