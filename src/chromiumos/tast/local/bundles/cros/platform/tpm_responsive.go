// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: TPMResponsive,
		Desc: "Checks that TPM is responsive",
		Contacts: []string{
			"apronin@chromium.org",
			"nya@chromium.org", // Tast port author
		},
		SoftwareDeps: []string{"tpm"},
		Attr:         []string{"group:mainline", "group:labqual"},
	})
}

func TPMResponsive(ctx context.Context, s *testing.State) {
	cmd := testexec.CommandContext(ctx, "tpm_version")
	out, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		s.Fatal("Failed to run tpm_version: ", err)
	}

	const (
		exp = "Version Info"
		fn  = "tpm_version.txt"
	)
	if !strings.Contains(string(out), exp) {
		s.Errorf("tpm_version output doesn't contain %q (see %v)", exp, fn)
	}
	if err := ioutil.WriteFile(filepath.Join(s.OutDir(), fn), out, 0644); err != nil {
		s.Error("Failed to write output: ", err)
	}
}
