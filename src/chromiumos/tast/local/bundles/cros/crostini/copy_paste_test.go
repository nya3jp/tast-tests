// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"testing"

	"chromiumos/tast/common/genparams"
	"chromiumos/tast/local/crostini"
)

func TestCopyPasteParams(t *testing.T) {
	params := crostini.MakeTestParamsFromList(t, []crostini.Param{
		{
			Name: "wayland_to_wayland",
			Val: `testParameters{
				Copy:  waylandCopyConfig,
				Paste: waylandPasteConfig,
			}`,
		},
		{
			Name: "wayland_to_x11",
			Val: `testParameters{
				Copy:  waylandCopyConfig,
				Paste: x11PasteConfig,
			}`,
		},
		{
			Name: "x11_to_wayland",
			Val: `testParameters{
				Copy:  x11CopyConfig,
				Paste: waylandPasteConfig,
			}`,
		},
		{
			Name: "x11_to_x11",
			Val: `testParameters{
				Copy:  x11CopyConfig,
				Paste: x11PasteConfig,
			}`,
		}})
	genparams.Ensure(t, "copy_paste.go", params)
}
