// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"time"

	"chromiumos/tast/local/crostini"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Sanity,
		Desc:         "Tests basic Crostini startup only (where crostini was shipped with the build)",
		Contacts:     []string{"smbarber@chromium.org", "cros-containers-dev@google.com"},
		SoftwareDeps: []string{"chrome", "vm_host"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{
			{
				Name:      "artifact",
				Pre:       crostini.StartedByArtifact(),
				Timeout:   7 * time.Minute,
				ExtraData: []string{crostini.ImageArtifact},
			},
			{
				Name:    "download",
				Pre:     crostini.StartedByDownload(),
				Timeout: 10 * time.Minute,
			},
			{
				Name:    "download_buster",
				Pre:     crostini.StartedByDownloadBuster(),
				Timeout: 10 * time.Minute,
			},
			{
				Name:      "installer",
				Pre:       crostini.StartedByInstaller(),
				Timeout:   7 * time.Minute,
				ExtraData: []string{crostini.ImageArtifact},
			},
		},
	})
}

func Sanity(ctx context.Context, s *testing.State) {
	pre := s.PreValue().(crostini.PreData)
	if !pre.Valid {
		s.Error("Crostini precondition is not valid")
		return
	}
	cont := pre.Container

	if err := crostini.SimpleCommandWorks(ctx, cont); err != nil {
		s.Fatal("Failed to run a command in the container: ", err)
	}
}
