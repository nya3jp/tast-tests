// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: TestPPDs,
		Desc: "Verifies the PPD files pass cupstestppd",
		Contacts: []string{
			"batrapranav@chromium.org",
			"cros-printing-dev@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"cros_internal", "cups"},
		Data:         []string{ppdsAll},
	})
}

const (
	ppdsAll = "ppds_all.tar.xz"
)

func TestPPDs(ctx context.Context, s *testing.State) {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		s.Fatal("Failed to create temp dir: ", err)
	}
	defer os.RemoveAll(dir)
	cmd := testexec.CommandContext(ctx, "tar", "-xJC", dir, "-f", s.DataPath(ppdsAll), "--strip-components=1")
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to extract archive: ", err)
	}
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		s.Fatal("Failed to read directory: ", err)
	}
	for _, file := range files {
		cmd := testexec.CommandContext(ctx, "cupstestppd", "-W", "translations", filepath.Join(dir, file.Name()))
		if err := cmd.Run(testexec.DumpLogOnError); err != nil {
			s.Errorf("%s: %v", file.Name(), err)
		}
	}
}
