// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"
	"io/ioutil"
	"strings"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     DataFiles,
		Desc:     "Demonstrates how to use data files",
		Contacts: []string{"derat@chromium.org", "tast-users@chromium.org"},
		Data: []string{
			"data_files_internal.txt",
			"data_files_external.txt",
		},
	})
}

func DataFiles(ctx context.Context, s *testing.State) {
	// Read a data file that's directly checked in to this repository in the data/ subdirectory.
	b, err := ioutil.ReadFile(s.DataPath("data_files_internal.txt"))
	if err != nil {
		s.Error("Failed reading internal data file: ", err)
	} else {
		s.Logf("Read internal data file: %q", strings.TrimRight(string(b), "\n"))
	}

	// Read a data file that's stored in Google Cloud Storage and linked by an external link
	// file (*.external) in the data/ subdirectory.
	if b, err = ioutil.ReadFile(s.DataPath("data_files_external.txt")); err != nil {
		s.Error("Failed reading external data file: ", err)
	} else {
		s.Logf("Read external data file: %q", strings.TrimRight(string(b), "\n"))
	}
}
