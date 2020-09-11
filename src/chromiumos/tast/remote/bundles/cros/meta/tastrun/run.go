// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package tastrun helps meta tests run the tast command.
package tastrun

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// Exec execs the tast command using supplied arguments.
// subcmd contains the subcommand to use, e.g. "list" or "run".
// flags contains subcommand-specific flags.
// patterns contains a list of patterns matching tests.
// stdout.txt and stderr.txt output files are written unconditionally.
func Exec(ctx context.Context, s *testing.State, subcmd string, flags, patterns []string) (stdout, stderr []byte, err error) {
	meta := s.Meta()
	if meta == nil {
		return nil, nil, errors.New("failed to get meta info from context")
	}

	args := append([]string{subcmd}, flags...)
	args = append(args, meta.RunFlags...)
	args = append(args, meta.Target)
	args = append(args, patterns...)
	cmd := exec.CommandContext(ctx, meta.TastPath, args...)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	s.Log("Running ", strings.Join(cmd.Args, " "))
	runErr := cmd.Run()

	if werr := ioutil.WriteFile(filepath.Join(s.OutDir(), "stdout.txt"), stdoutBuf.Bytes(), 0644); werr != nil {
		s.Error("Failed to save stdout: ", werr)
	}
	if werr := ioutil.WriteFile(filepath.Join(s.OutDir(), "stderr.txt"), stderrBuf.Bytes(), 0644); werr != nil {
		s.Error("Failed to save stderr: ", werr)
	}
	return stdoutBuf.Bytes(), stderrBuf.Bytes(), runErr
}

// TestError is a subset of testing.EntityError.
type TestError struct {
	Reason string `json:"reason"`
}

// TestResult is subset of run.EntityResult.
type TestResult struct {
	Name       string      `json:"name"`
	Errors     []TestError `json:"errors"`
	SkipReason string      `json:"skipReason"`
}

// ParseResultsJSON parses results.json inside dir.
func ParseResultsJSON(dir string) ([]TestResult, error) {
	var results []TestResult
	rf, err := os.Open(filepath.Join(dir, "results.json"))
	if err != nil {
		return nil, errors.Wrap(err, "couldn't open results file")
	}
	defer rf.Close()

	if err = json.NewDecoder(rf).Decode(&results); err != nil {
		return nil, errors.Wrapf(err, "couldn't decode results from %v", rf.Name())
	}
	return results, nil
}
