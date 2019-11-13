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
		Func:     CopyPaste,
		Desc:     "Test copy paste functionality",
		Contacts: []string{"sidereal@google.com", "cros-containers-dev@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		Data:     []string{copypaste.CopyApplet, copypaste.PasteApplet},
		// Test every combination of:
		//   * Source container via Download/DownloadBuster/Artifact
		//   * Copy from Wayland|X11
		//   * Copy to Wayland|X11
		// As of writing tast requires that parameters are written out in full as
		// static initialisers hence the big list.
		Params: []testing.Param{
			{
				Name: "wayland_to_wayland_download",
				Val: copypaste.TestParameters{
					Copy:  copypaste.WaylandCopyConfig,
					Paste: copypaste.WaylandPasteConfig,
				},
				Pre:     crostini.StartedByDownload(),
				Timeout: 10 * time.Minute,
			},
			{
				Name: "wayland_to_x11_download",
				Val: copypaste.TestParameters{
					Copy:  copypaste.WaylandCopyConfig,
					Paste: copypaste.X11PasteConfig,
				},
				Pre:     crostini.StartedByDownload(),
				Timeout: 10 * time.Minute,
			},
			{
				Name: "x11_to_wayland_download",
				Val: copypaste.TestParameters{
					Copy:  copypaste.X11CopyConfig,
					Paste: copypaste.WaylandPasteConfig,
				},
				Pre:     crostini.StartedByDownload(),
				Timeout: 10 * time.Minute,
			},
			{
				Name: "x11_to_x11_download",
				Val: copypaste.TestParameters{
					Copy:  copypaste.X11CopyConfig,
					Paste: copypaste.X11PasteConfig,
				},
				Pre:     crostini.StartedByDownload(),
				Timeout: 10 * time.Minute,
			},
			{
				Name: "wayland_to_wayland_download_buster",
				Val: copypaste.TestParameters{
					Copy:  copypaste.WaylandCopyConfig,
					Paste: copypaste.WaylandPasteConfig,
				},
				Pre:     crostini.StartedByDownloadBuster(),
				Timeout: 10 * time.Minute,
			},
			{
				Name: "wayland_to_x11_download_buster",
				Val: copypaste.TestParameters{
					Copy:  copypaste.WaylandCopyConfig,
					Paste: copypaste.X11PasteConfig,
				},
				Pre:     crostini.StartedByDownloadBuster(),
				Timeout: 10 * time.Minute,
			},
			{
				Name: "x11_to_wayland_download_buster",
				Val: copypaste.TestParameters{
					Copy:  copypaste.X11CopyConfig,
					Paste: copypaste.WaylandPasteConfig,
				},
				Pre:     crostini.StartedByDownloadBuster(),
				Timeout: 10 * time.Minute,
			},
			{
				Name: "x11_to_x11_download_buster",
				Val: copypaste.TestParameters{
					Copy:  copypaste.X11CopyConfig,
					Paste: copypaste.X11PasteConfig,
				},
				Pre:     crostini.StartedByDownloadBuster(),
				Timeout: 10 * time.Minute,
			},
			{
				Name: "wayland_to_wayland_artifact",
				Val: copypaste.TestParameters{
					Copy:  copypaste.WaylandCopyConfig,
					Paste: copypaste.WaylandPasteConfig,
				},
				Pre:       crostini.StartedByArtifact(),
				Timeout:   7 * time.Minute,
				ExtraData: []string{crostini.ImageArtifact},
			},
			{
				Name: "wayland_to_x11_artifact",
				Val: copypaste.TestParameters{
					Copy:  copypaste.WaylandCopyConfig,
					Paste: copypaste.X11PasteConfig,
				},
				Pre:       crostini.StartedByArtifact(),
				Timeout:   7 * time.Minute,
				ExtraData: []string{crostini.ImageArtifact},
			},
			{
				Name: "x11_to_wayland_artifact",
				Val: copypaste.TestParameters{
					Copy:  copypaste.X11CopyConfig,
					Paste: copypaste.WaylandPasteConfig,
				},
				Pre:       crostini.StartedByArtifact(),
				Timeout:   7 * time.Minute,
				ExtraData: []string{crostini.ImageArtifact},
			},
			{
				Name: "x11_to_x11_artifact",
				Val: copypaste.TestParameters{
					Copy:  copypaste.X11CopyConfig,
					Paste: copypaste.X11PasteConfig,
				},
				Pre:       crostini.StartedByArtifact(),
				Timeout:   7 * time.Minute,
				ExtraData: []string{crostini.ImageArtifact},
			},
		},
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

func CopyPaste(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(crostini.PreData)
	param := s.Param().(copypaste.TestParameters)
	copypaste.RunTest(ctx, s, pre.TestAPIConn, pre.Container, pre.Keyboard,
		param.Copy, param.Paste)
}
