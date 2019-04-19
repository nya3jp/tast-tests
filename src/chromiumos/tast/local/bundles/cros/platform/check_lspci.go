// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"regexp"
	"strings"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     CheckLspci,
		Desc:     "Checks that lspci command is giving proper output or not",
		Contacts: []string{"kasaiah.bogineni@intel.com"},
		Attr:     []string{"informational"},
	})
}

func CheckLspci(ctx context.Context, s *testing.State) {
	cmd := testexec.CommandContext(ctx, "lspci")
	output, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatalf("lspci failed and the Error is %v ", err)
	} else if strings.TrimSpace(string(output)) == "" {
		s.Fatalf("lspci output is empty")
	}

	// Additionally checking the output format
	// LSPCI sample output is 00:1b.0 Ethernet controller: Red Hat, Inc Virtio network device
	regex := "\\d+:\\w+.\\d\\s.*:\\s.*"
	re := regexp.MustCompile(regex)
	lspciOutput := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, driver := range lspciOutput {
		match := re.MatchString(driver)
		if !match {
			s.Errorf("[output] %s - [pattern] %s mismatch", driver, regex)
		}
	}
}
