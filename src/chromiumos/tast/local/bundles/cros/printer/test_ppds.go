// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"
	"io/ioutil"

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
	ppdsAll  = "ppds_all.tar.xz"
	ppdsPath = "ppds_all/"
)

func TestPPDs(ctx context.Context, s *testing.State) {
	defer func() {
		testexec.CommandContext(ctx, "rm", "-r", ppdsPath).Run(testexec.DumpLogOnError)
	}()
	cmd := testexec.CommandContext(ctx, "tar", "-xJf", s.DataPath(ppdsAll))
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to extract archive")
	}
	files, err := ioutil.ReadDir(ppdsPath)
	if err != nil {
		s.Fatal("Failed to read directory")
	}
	for _, file := range files {
		cmd := testexec.CommandContext(ctx, "cupstestppd", "-W", "translations", ppdsPath+file.Name())
		if err := cmd.Run(testexec.DumpLogOnError); err != nil {
			s.Error(file.Name())
		}
	}
}
