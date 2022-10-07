// Copyright 2019 The ChromiumOS Authors
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
		Func:         ArtifactFiles,
		Desc:         "Demonstrates how to use artifact data files",
		Contacts:     []string{"tast-owners@chromium.org", "nya@chromium.org"},
		BugComponent: "b:1034625",
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{"artifact_files_partial_metadata_json"},
	})
}

// ArtifactFiles tests if local test can download artifact files.
// Do not promote this test because there is no guarantee the file will exist in future.
// This test is only for demonstration purpose.
func ArtifactFiles(ctx context.Context, s *testing.State) {
	// Build artifacts can be used as an external data file and read with
	// s.DataPath just similarly as internal data files or static external data files.
	// However, this works for ChromeOS images built on official builders only;
	// on developer builds an error is raised.
	if b, err := ioutil.ReadFile(s.DataPath("artifact_files_partial_metadata_json")); err != nil {
		s.Error("Failed reading artifact external data file: ", err)
	} else {
		s.Logf("Read artifact external data file (%d bytes)", len(b))
	}
}
