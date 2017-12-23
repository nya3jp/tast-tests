// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"io/ioutil"
	"strings"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DataFiles,
		Desc: "Demonstrates how to use data files",
		Data: []string{
			"data_files_data1.txt",
		},
	})
}

func DataFiles(s *testing.State) {
	b, err := ioutil.ReadFile(s.DataPath("data_files_data1.txt"))
	if err != nil {
		s.Error(err)
	} else {
		s.Logf("Read data file: %s", strings.TrimRight(string(b), "\n"))
	}
}
