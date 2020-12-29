// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/ui/cuj"
	et "chromiumos/tast/local/bundles/cros/ui/everydaymultitaskingcuj"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TC03S1EverydayMultiTaskingCUJYtMusic,
		Desc:         "Measures the performance of everyday multi-tasking CUJ",
		Contacts:     []string{"mukai@chromium.org", "tclaiborne@chromium.org", "jane.yang@cienet.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"android_p", "chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      15 * time.Minute,
		Vars: []string{
			"mute",
			"ui.cuj_username",
			"ui.cuj_password",
		},
		Pre: cuj.LoginKeepState(),
		Params: []testing.Param{
			{
				Val: et.TestParameters{
					TabletMode: false,
					Bluetooth:  false,
				},
			},
		},
		Data: []string{"cca_ui.js"},
	})
}

func TC03S1EverydayMultiTaskingCUJYtMusic(ctx context.Context, s *testing.State) {
	et.Run(ctx, s, et.YoutubeMusic)
}
