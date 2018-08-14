// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"io"
	"os"
	"path/filepath"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RemoteFiles,
		Desc: "Helper test that uses data and output files",
		// This test is called by remote tests in the meta package.
		Attr: []string{"disabled"},
		Data: []string{"remote_files_data.txt"},
	})
}

func RemoteFiles(s *testing.State) {
	const fn = "remote_files_data.txt"

	s.Log("Copying ", fn)
	sf, err := os.Open(s.DataPath(fn))
	if err != nil {
		s.Fatal("Failed to open data file: ", err)
	}
	defer sf.Close()

	df, err := os.Create(filepath.Join(s.OutDir(), fn))
	if err != nil {
		s.Fatal("Failed to create output file: ", err)
	}
	defer df.Close()

	if _, err = io.Copy(df, sf); err != nil {
		s.Fatal("Failed copying file: ", err)
	}
}
