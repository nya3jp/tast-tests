// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"testing"

	"chromiumos/tast/common/genparams"
	"chromiumos/tast/local/crostini"
)

func TestSecureCopyPasteParams(t *testing.T) {
	params := crostini.MakeTestParamsFromList(t, []crostini.Param{
		{
			Name:      "copy_wayland",
			ExtraData: []string{"secure_copy.py"},
			Val: `secureCopyPasteConfig{
				backend: "wayland",
				app:     "secure_copy.py",
				action:  copying,
			}`,
		}, {
			Name:      "copy_x11",
			ExtraData: []string{"secure_copy.py"},
			Val: `secureCopyPasteConfig{
				backend: "x11",
				app:     "secure_copy.py",
				action:  copying,
			}`,
		}, {
			Name:      "paste_wayland",
			ExtraData: []string{"secure_paste.py"},
			Val: `secureCopyPasteConfig{
				backend: "wayland",
				app:     "secure_paste.py",
				action:  pasting,
			}`,
		}, {
			Name:      "paste_x11",
			ExtraData: []string{"secure_paste.py"},
			Val: `secureCopyPasteConfig{
				backend: "x11",
				app:     "secure_paste.py",
				action:  pasting,
			}`,
		}})
	genparams.Ensure(t, "secure_copy_paste.go", params)
}
