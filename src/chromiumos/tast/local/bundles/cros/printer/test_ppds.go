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
	"runtime"
	"sort"
	"sync"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: TestPPDs,
		Desc: "Verifies the PPD files pass cupstestppd and foomatic-rip",
		Contacts: []string{
			"batrapranav@chromium.org",
			"cros-printing-dev@chromium.org",
		},
		Attr: []string{
			"group:mainline",
			"informational",
			"group:paper-io",
			"paper-io_printing",
		},
		SoftwareDeps: []string{"cros_internal", "cups"},
		Data:         []string{ppdsAll},
	})
}

const ppdsAll = "ppds_all.tar.xz"

// fooRip detects if the PPD file uses the foomatic-rip print filter.
var fooRip = regexp.MustCompile(`foomatic-rip"`)

// fooCmd is the actual shell command (garbled) that foomatic filter executes.
// Two PPD files with the same fooCmd will tend to execute the same shell commands
// to generate print data. Note that setting the environment variable
// FOOMATIC_VERIFY_MODE prevents shell commands from being executed by the
// platform2 foomatic shell while still verifying that they are valid.
var fooCmd = regexp.MustCompile(`(?m)^\*FoomaticRIPCommandLine: "[^"]*"`)

type fileError struct {
	file string
	err  error
}

// testPPD creates temp file ppdFile with contents ppdContents and
// tests it for validity. ppdFile is deleted before returning.
func testPPD(ctx context.Context, fooCache *sync.Map, ppdFile string, ppdContents []byte) error {
	const pdf = `%PDF-1.0
1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj 2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj 3 0 obj<</Type/Page/MediaBox[0 0 3 3]>>endobj
xref
0 4
0000000000 65535 f 
0000000009 00000 n 
0000000052 00000 n 
0000000101 00000 n 
trailer<</Size 4/Root 1 0 R>>
startxref
147
%EOF`
	if err := ioutil.WriteFile(ppdFile, ppdContents, 0644); err != nil {
		return errors.Wrap(err, "failed to write file")
	}
	defer os.Remove(ppdFile)
	// cupstestppd verifies that the PPD file is valid.
	// -W translations ignores translations strings as these are not used by the
	// Chrome OS version of CUPS. Debugd also calls cupstestppd with this flag.
	cmd := testexec.CommandContext(ctx, "cupstestppd", "-W", "translations", ppdFile)
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "cupstestppd")
	}
	// Check if the PPD file uses the foomatic-rip filter.
	if !fooRip.Match(ppdContents) {
		return nil
	}
	cmds := fooCmd.FindAll(ppdContents, 2)
	if len(cmds) > 1 {
		return errors.New("multiple FoomaticRIPCommandLine matches")
	}
	id := ""
	if len(cmds) == 1 {
		id = string(cmds[0])
	}
	if fooCache != nil {
		val, ok := fooCache.Load(id)
		if ok {
			if val != "" {
				return errors.Errorf("foomatic-rip: same error as %q", val)
			}
			return nil
		}
	}
	cmd = testexec.CommandContext(ctx, "foomatic-rip", "1" /*jobID*/, "chronos" /*user*/, "Untitled" /*title*/, "1" /*copies*/, "" /*options*/)
	cmd.Env = []string{"FOOMATIC_VERIFY_MODE=true",
		"PATH=/bin:/usr/bin:/usr/libexec/cups/filter",
		"PPD=" + ppdFile}
	cmd.Stdin = bytes.NewReader([]byte(pdf))
	err := cmd.Run(testexec.DumpLogOnError)
	if fooCache != nil {
		val := ""
		if err != nil {
			val = filepath.Base(ppdFile)
		}
		fooCache.Store(id, val)
	}
	if err != nil {
		return errors.Wrap(err, "foomatic-rip")
	}
	return nil
}

// extractPPD extracts a gzip compressed PPD file and returns its contents.
func extractPPD(ctx context.Context, ppd []byte) ([]byte, error) {
	buf, err := gzip.NewReader(bytes.NewReader(ppd))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create reader")
	}
	ppd, readErr := ioutil.ReadAll(buf)
	if err := buf.Close(); err != nil {
		testing.ContextLog(ctx, "Failed to close gzip: ", err)
	}
	if readErr != nil {
		return nil, errors.Wrap(readErr, "failed to read gzip")
	}
	return ppd, nil
}

// testPPDs receives PPD files to test (located in dir) through the
// files channel and returns any errors through the errors channel.
func testPPDs(ctx context.Context, dir string, files <-chan string, errors chan<- []fileError, fooCache *sync.Map) {
	const gzExt = ".gz"
	var errs []fileError
	for file := range files {
		if err := func() error {
			ppd, err := ioutil.ReadFile(filepath.Join(dir, file))
			if err != nil {
				return err
			}
			if filepath.Ext(file) == gzExt {
				ppd, err = extractPPD(ctx, ppd)
				if err != nil {
					return err
				}
				file = file[:len(file)-len(gzExt)]
			}
			return testPPD(ctx, fooCache, filepath.Join(dir, file), ppd)
		}(); err != nil {
			errs = append(errs, fileError{file, err})
		}
	}
	errors <- errs
}

// extractArchive extracts a tar.xz archive via the tar command.
func extractArchive(ctx context.Context, src, dst string) error {
	return testexec.CommandContext(ctx, "tar", "-xC", dst, "-f", src, "--strip-components=1").Run(testexec.DumpLogOnError)
}

func TestPPDs(ctx context.Context, s *testing.State) {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		s.Fatal("Failed to create temp dir: ", err)
	}
	defer os.RemoveAll(dir)
	// ppds_all.tar.xz takes around 60M when decompressed.
	if err := extractArchive(ctx, s.DataPath(ppdsAll), dir); err != nil {
		s.Fatal("Failed to extract archive: ", err)
	}
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		s.Fatal("Failed to read directory: ", err)
	}
	n := len(files)
	// Stride must be a power of 2.
	// It is used to prevent fooCache cache misses by spacing out the
	// lexicographically sorted files so that they are no longer in
	// lexicographical order.
	const stride = 1 << 6
	if n <= stride+1 {
		s.Fatalf("Too few files: %d found", n)
	}
	input := make(chan string, n)
	// Always use at least 2 goroutines in case one of them is blocked.
	numRoutines := runtime.NumCPU() + 1
	testing.ContextLogf(ctx, "Creating %d goroutines", numRoutines)
	errors := make(chan []fileError, numRoutines)
	// fooCache maps fooCmd to an empty string if foomatic-rip succeeds and if it fails,
	// the name of the PPD file on which it failed.
	var fooCache *sync.Map
	// Disabling cacheFoo roughly triples the runtime.
	const cacheFoo = true
	if cacheFoo {
		fooCache = new(sync.Map)
	}
	for i := 0; i < numRoutines; i++ {
		go testPPDs(ctx, dir, input, errors, fooCache)
	}
	// Ensure n is coprime to stride so that each file is selected exactly once.
	if n&1 == 0 {
		n--
		input <- files[n].Name()
	}
	// Space the files out to avoid fooCache cache misses.
	for i := 0; i != n; i += stride {
		if i > n {
			i -= n
		}
		input <- files[i].Name()
	}
	close(input)

	var errs []fileError
	for i := 0; i < numRoutines; i++ {
		errs = append(errs, <-errors...)
	}
	sort.Slice(errs, func(i, j int) bool { return errs[i].file < errs[j].file })
	for _, fileErr := range errs {
		s.Errorf("%s: %v", fileErr.file, fileErr.err)
	}
}
