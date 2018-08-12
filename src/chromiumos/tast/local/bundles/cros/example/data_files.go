// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"io/ioutil"
	"strings"

	"chromiumos/tast/local/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DataFiles,
		Desc: "Demonstrates how to use data files",
		Attr: []string{"informational"},
		Data: []string{
			"data_files_internal.txt",
			"data_files_external.txt",
		},
	})
}

func DataFiles(s *testing.State) {
	defer faillog.SaveIfError(s)

	// Read a data file that's checked in to this repository in the data/ subdirectory.
	b, err := ioutil.ReadFile(s.DataPath("data_files_internal.txt"))
	if err != nil {
		s.Error("Failed reading internal data file: ", err)
	} else {
		s.Logf("Read internal data file: %q", strings.TrimRight(string(b), "\n"))
	}

	// Read a data file that's stored in Google Cloud Storage and declared via the
	// external_data.txt file in the tast-local-tests-cros package.
	if b, err = ioutil.ReadFile(s.DataPath("data_files_external.txt")); err != nil {
		s.Error("Failed reading external data file: ", err)
	} else {
		s.Logf("Read external data file: %q", strings.TrimRight(string(b), "\n"))
	}
}
