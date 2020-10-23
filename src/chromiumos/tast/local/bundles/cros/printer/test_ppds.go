// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"bytes"
	"compress/gzip"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"

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
		Data:         []string{"to_print.pdf", ppdsAll},
	})
}

const (
	ppdsAll = "ppds_all.tar.xz"
)

func TestPPDs(ctx context.Context, s *testing.State) {
	os.Setenv("FOOMATIC_VERIFY_MODE", "true")
	os.Setenv("PATH", "/bin:/usr/bin:/usr/libexec/cups/filter")
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		s.Fatal("Failed to create temp dir: ", err)
	}
	os.Setenv("PPD", filepath.Join(dir, "ppd.ppd"))
	defer os.RemoveAll(dir)
	// ppds_all.tar.xz takes around 60M when decompressed.
	cmd := testexec.CommandContext(ctx, "tar", "-xJC", dir, "-f", s.DataPath(ppdsAll), "--strip-components=1")
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to extract archive: ", err)
	}
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		s.Fatal("Failed to read directory: ", err)
	}
	fooCmd := regexp.MustCompile(`(?m)^\*FoomaticRIPCommandLine: "[^"]*"`)
	fooMap := make(map[string]string)
	for _, file := range files {
		cmd := testexec.CommandContext(ctx, "cupstestppd", "-W", "translations", filepath.Join(dir, file.Name()))
		if err := cmd.Run(testexec.DumpLogOnError); err != nil {
			s.Errorf("%s: %v", file.Name(), err)
			continue
		}
		ppd, err := ioutil.ReadFile(filepath.Join(dir, file.Name()))
		if err != nil {
			s.Fatal("Failed to read PPD file: ", err)
		}
		if filepath.Ext(file.Name()) == ".gz" {
			buf, err := gzip.NewReader(bytes.NewReader(ppd))
			if err != nil {
				s.Fatal("Failed to read gzip: ", err)
			}
			ppd, err = ioutil.ReadAll(buf)
			if err != nil {
				s.Fatal("Failed to read gzip: ", err)
			}
		}
		cmds := fooCmd.FindAll(ppd, 2)
		if len(cmds) == 2 {
			s.Errorf("%s: Multiple FoomaticRIPCommandLine matches", file.Name())
		}
		if len(cmds) == 1 {
			id := string(cmds[0])
			if val, ok := fooMap[id]; ok {
				if val != "" {
					s.Errorf("%s: foomatic-rip: same error as %q", file.Name(), val)
				}
				continue
			}
			fooMap[id] = ""
			ioutil.WriteFile(filepath.Join(dir, "ppd.ppd"), ppd, 0644)
			cmd := testexec.CommandContext(ctx, "foomatic-rip", "1" /*jobID*/, "chronos" /*user*/, "Untitled" /*title*/, "1" /*copies*/, "" /*options*/, s.DataPath("to_print.pdf"))
			if err := cmd.Run(testexec.DumpLogOnError); err != nil {
				s.Errorf("%s: foomatic-rip: %v", file.Name(), err)
				fooMap[id] = file.Name()
			}
		}
	}
}
