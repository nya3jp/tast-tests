// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"

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
	ppdsAll  = "ppds_all.tar.xz"
	cacheFoo = true
)

var fooCmd = regexp.MustCompile(`(?m)^\*FoomaticRIPCommandLine: "[^"]*"`)
var fooMap = make(map[string]string)
var mutex sync.RWMutex

func testPPDs(ctx context.Context, s *testing.State, dir, ppdFile string, files chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	ppdFile = filepath.Join(dir, ppdFile)
	env := []string{"FOOMATIC_VERIFY_MODE=true",
		"PATH=/bin:/usr/bin:/usr/libexec/cups/filter",
		"PPD=" + ppdFile}
	for file := range files {
		ppd, err := ioutil.ReadFile(filepath.Join(dir, file))
		if err != nil {
			s.Fatal("Failed to read PPD file: ", err)
		}
		if filepath.Ext(file) == ".gz" {
			buf, err := gzip.NewReader(bytes.NewReader(ppd))
			if err != nil {
				s.Fatal("Failed to create reader: ", err)
			}
			ppd, err = ioutil.ReadAll(buf)
			if err := buf.Close(); err != nil {
				s.Error("Failed to close gzip: ", err)
			}
			if err != nil {
				s.Fatal("Failed to read gzip: ", err)
			}
		}
		ioutil.WriteFile(ppdFile, ppd, 0644)
		cmd := testexec.CommandContext(ctx, "cupstestppd", "-W", "translations", ppdFile)
		if err := cmd.Run(testexec.DumpLogOnError); err != nil {
			s.Errorf("%s: %v", file, err)
			continue
		}
		cmds := fooCmd.FindAll(ppd, 2)
		if len(cmds) > 1 {
			s.Errorf("%s: Multiple FoomaticRIPCommandLine matches", file)
		}
		if len(cmds) == 1 {
			id := string(cmds[0])
			if cacheFoo {
				mutex.RLock()
				val, ok := fooMap[id]
				mutex.RUnlock()
				if ok {
					if val != "" {
						s.Errorf("%s: foomatic-rip: same error as %q", file, val)
					}
					continue
				}
			}
			cmd := testexec.CommandContext(ctx, "foomatic-rip", "1" /*jobID*/, "chronos" /*user*/, "Untitled" /*title*/, "1" /*copies*/, "" /*options*/, s.DataPath("to_print.pdf"))
			cmd.Env = env
			err := cmd.Run(testexec.DumpLogOnError)
			if err != nil {
				s.Errorf("%s: foomatic-rip: %v", file, err)
			}
			if cacheFoo {
				mutex.Lock()
				fooMap[id] = ""
				if err != nil {
					fooMap[id] = file
				}
				mutex.Unlock()
			}
		}
	}
}

func TestPPDs(ctx context.Context, s *testing.State) {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		s.Fatal("Failed to create temp dir: ", err)
	}
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
	input := make(chan string)
	var wg sync.WaitGroup
	numCPU := runtime.NumCPU()
	s.Logf("Found %d CPUs", numCPU)
	for i := 0; i < numCPU; i++ {
		wg.Add(1)
		go testPPDs(ctx, s, dir, fmt.Sprintf("ppd%d.ppd", i), input, &wg)
	}
	n := len(files)
	if n < 100 {
		s.Fatalf("Too few files: %d found", n)
	}
	if n&1 == 0 {
		n--
		input <- files[n].Name()
	}
	// Space the files out to avoid fooMap cache misses.
	for i := 0; i != n; i += 64 {
		if i > n {
			i -= n
		}
		input <- files[i].Name()
	}
	close(input)
	wg.Wait()
}
