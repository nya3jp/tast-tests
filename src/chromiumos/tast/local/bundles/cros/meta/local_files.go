// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"
	"path/filepath"

	"chromiumos/tast/fsutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     LocalFiles,
		Desc:     "Helper test that uses data and output files",
		Contacts: []string{"derat@chromium.org"},
		// This test is executed by remote tests in the meta package.
		Attr: []string{"disabled"},
		Data: []string{
			"local_files_internal.txt",
			"local_files_external.txt",
		},
	})
}

func LocalFiles(ctx context.Context, s *testing.State) {
	for _, fn := range []string{
		"local_files_internal.txt",
		"local_files_external.txt",
	} {
		s.Log("Copying ", fn)
		if err := fsutil.CopyFile(s.DataPath(fn), filepath.Join(s.OutDir(), fn)); err != nil {
			s.Errorf("Failed copying %s: %s", fn, err)
		}
	}
}
