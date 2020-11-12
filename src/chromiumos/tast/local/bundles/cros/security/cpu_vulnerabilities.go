// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"strings"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CPUVulnerabilities,
		Desc: "Confirm CPU vulnerabilities are mitigated",
		Contacts: []string{
			"swboyd@chromium.org", // Tast author
			"chromeos-security@google.com",
		},
		Attr: []string{"group:mainline", "informational"},
	})
}

func CPUVulnerabilities(ctx context.Context, s *testing.State) {
	vulnDir := "/sys/devices/system/cpu/vulnerabilities/"
	fileList, err := ioutil.ReadDir(vulnDir)
	if err != nil {
		s.Fatal("Failed to list vulnerability files: ", err)
	}
	var vulnerable []string
	for _, f := range fileList {
		fName := f.Name()
		bcontents, err := ioutil.ReadFile(filepath.Join(vulnDir, fName))
		if err != nil {
			s.Fatal("Can't read vulnerability file: ", err)
		}
		contents := strings.TrimSpace(string(bcontents))
		s.Logf("%s: %s", fName, contents)
		if strings.EqualFold(contents, "vulnerable") {
			vulnerable = append(vulnerable, fName)
		}
	}
	if len(vulnerable) > 0 {
		s.Fatal("Kernel has CPU vulnerabilities: ", vulnerable)
	}
}
