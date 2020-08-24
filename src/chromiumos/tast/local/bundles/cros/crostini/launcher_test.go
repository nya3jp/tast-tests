// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"testing"

	"chromiumos/tast/common/genparams"
	"chromiumos/tast/local/crostini"
)

func TestLauncherParams(t *testing.T) {
	params := crostini.MakeTestParamsFromList(t, []crostini.Param{
		{
			Name:      "local_wayland",
			ExtraData: []string{"launcher_wayland_demo_fixed_size.desktop", "launcher_wayland_demo.png"},
			Val: `launcherTestConfig{
				desktopFile: "wayland_demo_fixed_size.desktop",
				iconFile:    "wayland_demo.png",
				windowName:  "wayland_demo_fixed_size",
				installRoot: "/home/testuser/.local",
				launcherID:  "ddlengdehbebnlegdnllbdhpjofodekl",
			}`,
		}, {
			Name:      "local_x11",
			ExtraData: []string{"launcher_x11_demo_fixed_size.desktop", "launcher_x11_demo.png"},
			Val: `launcherTestConfig{
				desktopFile: "x11_demo_fixed_size.desktop",
				iconFile:    "x11_demo.png",
				windowName:  "x11_demo_fixed_size",
				installRoot: "/home/testuser/.local",
				launcherID:  "mddfmcdnhpnhoefmmiochnnjofmfhanb",
			}`,
		}, {
			Name:      "system_wayland",
			ExtraData: []string{"launcher_wayland_demo_fixed_size.desktop", "launcher_wayland_demo.png"},
			Val: `launcherTestConfig{
				desktopFile: "wayland_demo_fixed_size.desktop",
				iconFile:    "wayland_demo.png",
				windowName:  "wayland_demo_fixed_size",
				installRoot: "/usr",
				launcherID:  "ddlengdehbebnlegdnllbdhpjofodekl",
			}`,
		}, {
			Name:      "system_x11",
			ExtraData: []string{"launcher_x11_demo_fixed_size.desktop", "launcher_x11_demo.png"},
			Val: `launcherTestConfig{
				desktopFile: "x11_demo_fixed_size.desktop",
				iconFile:    "x11_demo.png",
				windowName:  "x11_demo_fixed_size",
				installRoot: "/usr",
				launcherID:  "mddfmcdnhpnhoefmmiochnnjofmfhanb",
			}`,
		}})
	genparams.Ensure(t, "launcher.go", params)
}
