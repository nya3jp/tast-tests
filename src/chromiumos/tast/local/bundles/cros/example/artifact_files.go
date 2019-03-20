// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"
	"io/ioutil"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ArtifactFiles,
		Desc:     "Demonstrates how to use artifact data files",
		Contacts: []string{"nya@chromium.org", "tast-users@chromium.org"},
		Attr:     []string{"informational"},
		Data:     []string{"artifact_files_license_credits.html"},
	})
}

func ArtifactFiles(ctx context.Context, s *testing.State) {
	// Build artifacts can be used as an external data file and read with
	// s.DataPath just similarly as internal data files or static external data files.
	// However, this works for Chrome OS images built on official builders only;
	// on developer builds an error is raised.
	if b, err := ioutil.ReadFile(s.DataPath("artifact_files_license_credits.html")); err != nil {
		s.Error("Failed reading artifact external data file: ", err)
	} else {
		s.Logf("Read artifact external data file (%d bytes)", len(b))
	}
}
