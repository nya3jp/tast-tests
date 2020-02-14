// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/local/crostini"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UninstallInvalidApp,
		Desc:         "Attempts to uninstall a non-existant desktop file and expects to see errors",
		Contacts:     []string{"smbarber@chromium.org", "cros-containers-dev@google.com"},
		Attr:         []string{"group:mainline"},
		Timeout:      7 * time.Minute,
		Data:         []string{crostini.ImageArtifact},
		Pre:          crostini.StartedByArtifact(),
		SoftwareDeps: []string{"chrome", "vm_host"},
		Params: []testing.Param{
			{
				Name:              "artifact",
				ExtraSoftwareDeps: []string{"crostini_stable"},
			},
			{
				Name:              "artifact_unstable",
				ExtraSoftwareDeps: []string{"crostini_unstable"},
				ExtraAttr:         []string{"informational"},
			},
		},
	})
}

func UninstallInvalidApp(ctx context.Context, s *testing.State) {
	cont := s.PreValue().(crostini.PreData).Container

	err := cont.UninstallPackageOwningFile(ctx, "bad")
	if err == nil {
		s.Error("Did not fail when attempting invalid UninstallPackageOwningFile")
		return
	}
	if !strings.Contains(err.Error(), "desktop_file_id does not exist") {
		s.Error("Did not get expected error messages when running invalid UninstallPackageOwningFile: ", err)
	}
}
