// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"io/ioutil"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     RemoteArtifactFiles,
		Desc:     "Demonstrates how to use artifact data files for remote test",
		Contacts: []string{"seewaifu@chromium.org", "tast-owners@chromium.org"},
		Attr:     []string{"group:mainline", "informational"},
		Data:     []string{"artifact_files_UPLOADED"},
	})
}

// RemoteArtifactFiles tests if build artifacts can be download from remote tests.
func RemoteArtifactFiles(ctx context.Context, s *testing.State) {
	// Build artifacts can be used as an external data file and read with
	// s.DataPath just similarly as internal data files or static external data files.
	// However, this works for ChromeOS images built on official builders only;
	// on developer builds an error is raised.
	if b, err := ioutil.ReadFile(s.DataPath("artifact_files_UPLOADED")); err != nil {
		s.Error("Failed reading artifact external data file: ", err)
	} else {
		s.Logf("Read artifact external data file (%d bytes)", len(b))
	}
}
