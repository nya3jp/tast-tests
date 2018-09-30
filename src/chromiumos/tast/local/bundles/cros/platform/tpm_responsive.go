// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TPMResponsive,
		Desc:         "Checks that TPM is responsive",
		SoftwareDeps: []string{"tpm"},
	})
}

func TPMResponsive(ctx context.Context, s *testing.State) {
	cmd := testexec.CommandContext(ctx, "tpm_version")
	out, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		s.Fatal("Failed to run tpm_version: ", err)
	}

	if !strings.Contains(string(out), "Version Info") {
		s.Error("Unexpected tpm_version output:\n", string(out))
	}
}
