// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/crostini/copypaste"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     CopyPasteDownload,
		Desc:     "Test copy paste functionality (where crostini was downloaded first)",
		Contacts: []string{"sidereal@google.com", "cros-containers-dev@google.com"},
		Attr:     []string{"informational"},
		Data:     []string{copypaste.CopyApplet, copypaste.PasteApplet},
		Params: []testing.Param{
			{
				Name: "wayland_to_wayland",
				Val: copypaste.TestParameters{
					Copy:  copypaste.WaylandCopyConfig,
					Paste: copypaste.WaylandPasteConfig,
				},
			},
			{
				Name: "wayland_to_x11",
				Val: copypaste.TestParameters{
					Copy:  copypaste.WaylandCopyConfig,
					Paste: copypaste.X11PasteConfig,
				},
			},
			{
				Name: "x11_to_wayland",
				Val: copypaste.TestParameters{
					Copy:  copypaste.X11CopyConfig,
					Paste: copypaste.WaylandPasteConfig,
				},
			},
			{
				Name: "x11_to_x11",
				Val: copypaste.TestParameters{
					Copy:  copypaste.X11CopyConfig,
					Paste: copypaste.X11PasteConfig,
				},
			},
		},
		Pre:          crostini.StartedByDownload(),
		Timeout:      10 * time.Minute,
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

func CopyPasteDownload(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(crostini.PreData)
	param := s.Param().(copypaste.TestParameters)
	copypaste.RunTest(ctx, s, pre.TestAPIConn, pre.Container,
		param.Copy, param.Paste)
}
