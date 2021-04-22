// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ghostscript provides common utilities for testing ghostscript
// filters.
package ghostscript

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/printing/document"
	"chromiumos/tast/testing"
)

// RunTest runs a ghostscript filter given by gsFilter and verifies that the
// output produced by running the given input file through the filter matches
// the contents of the given golden file. Some filters may require an extra
// environment variable to be set, which is given by envVar.
func RunTest(ctx context.Context, s *testing.State, gsFilter, input, golden, envVar string) {
	inputContents, err := ioutil.ReadFile(input)
	if err != nil {
		s.Fatal("Failed to load file contents: ", err)
	}

	commandPath := "/usr/libexec/cups/filter/" + gsFilter
	cmd := testexec.CommandContext(ctx, commandPath, "1" /*jobID*/, "chronos" /*user*/, "Untitled" /*title*/, "1" /*copies*/, "" /*options*/)

	// Add the given envVar to the command if it's not empty.
	if envVar != "" {
		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, envVar)
	}

	// Capture a pipe to the stdin of the ghostscript filter.
	stdin, err := cmd.StdinPipe()
	if err != nil {
		s.Fatalf("Failed to open stdin pipe for %s command: %v", gsFilter, err)
	}

	// Pass the contents of the given input file into the ghostscript filter using
	// the stdin pipe.
	go func() {
		defer stdin.Close()
		if _, err := stdin.Write(inputContents); err != nil {
			s.Errorf("Failed to write to stdin pipe for %s command: %v", gsFilter, err)
		}
	}()

	output, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		s.Fatalf("Failed to run %s command: %v", gsFilter, err)
	}

	goldenBytes, err := ioutil.ReadFile(golden)
	if err != nil {
		s.Fatalf("Failed to read file %s: %v", golden, err)
	}

	if document.CleanContents(string(goldenBytes)) != document.CleanContents(string(output)) {
		cmd.DumpLog(ctx)
		outFile := filepath.Base(golden)
		outPath := filepath.Join(s.OutDir(), outFile)
		if err := ioutil.WriteFile(outPath, output, 0644); err != nil {
			s.Error("Failed to dump output: ", err)
		}
		s.Errorf("Output differs from expected: output saved to %q", outFile)
	}
}
