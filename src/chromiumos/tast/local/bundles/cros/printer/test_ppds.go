// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"

	"chromiumos/tast/errors"
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
	ppdsAll = "ppds_all.tar.xz"
	// Disabling cacheFoo triples the runtime.
	cacheFoo   = true
	numThreads = 16
)

// fooRip detects if the PPD file uses the foomatic-rip print filter.
var fooRip = regexp.MustCompile(`foomatic-rip"`)

// fooCmd is the actual shell command (garbled) that foomatic filter executes.
// Two PPD files with the same fooCmd will tend to execute the same shell commands
// to generate print data. Note that setting the environment variable
// FOOMATIC_VERIFY_MODE prevents shell commands from being executed by the
// platform2 foomatic shell while still verifying that they are valid.
var fooCmd = regexp.MustCompile(`(?m)^\*FoomaticRIPCommandLine: "[^"]*"`)

// fooCache maps fooCmd to an empty string if foomatic-rip succeeds and if it fails,
// the name of the PPD file on which it failed.
var fooCache = sync.Map{}

type fileError struct {
	file string
	err  error
}

// testPPD creates temp file ppdFile with contents ppdContents and
// tests it for validity. ppdFile is deleted before returning.
func testPPD(ctx context.Context, ppdFile string, ppdContents []byte) error {
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
		return errors.Errorf("failed to write file: %w", err)
	}
	defer os.Remove(ppdFile)
	cmd := testexec.CommandContext(ctx, "cupstestppd", "-W", "translations", ppdFile)
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		return errors.Errorf("cupstestppd: %w", err)
	}
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
	if cacheFoo {
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
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return errors.Errorf("failed to create stdin pipe: %w", err)
	}
	go func() {
		defer stdin.Close()
		io.WriteString(stdin, pdf)
	}()
	err = cmd.Run(testexec.DumpLogOnError)
	if cacheFoo {
		val := ""
		if err != nil {
			val = filepath.Base(ppdFile)
		}
		fooCache.Store(id, val)
	}
	if err != nil {
		return errors.Errorf("foomatic-rip: %w", err)
	}
	return nil
}

// extractPPD extracts a gzip compressed PPD file and returns its contents.
func extractPPD(ctx context.Context, ppd []byte) ([]byte, error) {
	buf, err := gzip.NewReader(bytes.NewReader(ppd))
	if err != nil {
		return nil, errors.Errorf("failed to create reader: %w", err)
	}
	ppd, err = ioutil.ReadAll(buf)
	if err := buf.Close(); err != nil {
		testing.ContextLog(ctx, "Failed to close gzip: ", err)
	}
	if err != nil {
		return nil, errors.Errorf("failed to read gzip: %w", err)
	}
	return ppd, nil
}

// testPPDs receives PPD files to test (located in dir) through the
// files channel and returns any errors through the errors channel.
func testPPDs(ctx context.Context, dir string, files chan string, errors chan []fileError, wg *sync.WaitGroup) {
	defer wg.Done()
	var errs []fileError
	for file := range files {
		ppd, err := ioutil.ReadFile(filepath.Join(dir, file))
		if err == nil && filepath.Ext(file) == ".gz" {
			ppd, err = extractPPD(ctx, ppd)
			file = file[:len(file)-3]
		}
		if err == nil {
			err = testPPD(ctx, filepath.Join(dir, file), ppd)
		}
		if err != nil {
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
	input := make(chan string)
	errors := make(chan []fileError)
	var wg sync.WaitGroup
	for i := 0; i < numThreads; i++ {
		wg.Add(1)
		go testPPDs(ctx, dir, input, errors, &wg)
	}
	n := len(files)
	if n < 100 {
		s.Fatalf("Too few files: %d found", n)
	}
	// Ensure n is odd.
	if n&1 == 0 {
		n--
		input <- files[n].Name()
	}
	// Space the files out to avoid fooCache cache misses.
	for i := 0; i != n; i += 64 {
		if i > n {
			i -= n
		}
		input <- files[i].Name()
	}
	close(input)

	var errs []fileError
	for i := 0; i < numThreads; i++ {
		errs = append(errs, <-errors...)
	}
	sort.Slice(errs, func(i, j int) bool { return errs[i].file < errs[j].file })
	for _, err := range errs {
		s.Errorf("%s: %v", err.file, err.err)
	}
	wg.Wait()
}
