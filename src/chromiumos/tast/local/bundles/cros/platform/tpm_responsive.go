// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"strings"

	"chromiumos/tast/local/faillog"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TPMResponsive,
		Desc:         "Checks that TPM is responsive",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"tpm"},
	})
}

func TPMResponsive(s *testing.State) {
	defer faillog.SaveIfError(s)

	cmd := testexec.CommandContext(s.Context(), "tpm_version")
	out, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(s.Context())
		s.Fatal("Failed to run tpm_version: ", err)
	}

	if !strings.Contains(string(out), "Version Info") {
		s.Error("Unexpected tpm_version output:\n", string(out))
	}
}
