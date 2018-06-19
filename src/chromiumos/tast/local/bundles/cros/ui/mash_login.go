// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"

	"github.com/shirou/gopsutil/process"
)

const (
	// Switch that identifies the ash mojo service. Keep in sync with chromium
	// switches::kMashServiceName in src/chrome/common/chrome_switches.cc
	mashServiceName = "mash-service-name"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MashLogin,
		Desc:         "Checks that chrome --enable-features=Mash starts",
		Attr:         []string{"bvt"},
		SoftwareDeps: []string{"chrome_login"},
	})
}

// MashLogin checks that chrome --enable-features=Mash starts and at least one mash service is running.
func MashLogin(s *testing.State) {
	cr, err := chrome.New(s.Context(), chrome.MashEnabled())
	if err != nil {
		saveFile := func(p string) error {
			sf, err := os.Open(p)
			if err != nil {
				return err
			}
			defer sf.Close()

			df, err := os.Create(filepath.Join(s.OutDir(), filepath.Base(p)))
			if err != nil {
				return err
			}
			defer df.Close()

			_, err = io.Copy(df, sf)
			return err
		}

		// TODO(crbug.com/850139): Stop collecting these files after fixing IsGuestSessionAllowed segfaults.
		ps, _ := filepath.Glob("/var/lib/whitelist/policy.*")
		for _, p := range append(ps, "/home/chronos/Local State") {
			if err = saveFile(p); err != nil {
				s.Errorf("Failed to save %s: %v", p, err)
			}
		}

		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(s.Context())

	pids, err := chrome.GetPIDs()
	if err != nil {
		s.Fatal("Could not get chrome PIDs: ", err)
	}

	found := false
	cmds := make([]string, 0)
	for _, pid := range pids {
		// If we see errors, assume the process exited.
		proc, err := process.NewProcess(int32(pid))
		if err != nil {
			continue
		}
		cmd, err := proc.Cmdline()
		if err != nil {
			continue
		}
		if strings.Contains(cmd, mashServiceName) {
			found = true
			break
		}
		cmds = append(cmds, cmd)
	}
	if !found {
		s.Errorf("No chrome process containing %q among %v", mashServiceName, cmds)
	}
}
