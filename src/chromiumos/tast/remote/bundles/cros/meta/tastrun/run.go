// Copyright 2018 The ChromiumOS Authors
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

// NewCommand creates a command to run tast.
// subcmd contains the subcommand to use, e.g. "list" or "run".
// flags contains subcommand-specific flags.
// patterns contains a list of patterns matching tests.
func NewCommand(ctx context.Context, s *testing.State, subcmd string, flags, patterns []string) (*exec.Cmd, error) {
	meta := s.Meta()
	if meta == nil {
		return nil, errors.New("failed to get meta info from context")
	}

	args := append([]string{subcmd}, flags...)
	switch subcmd {
	case "run":
		args = append(args, meta.RunFlags...)
		// We're already in a Tast test; don't redo the system log
		// collection. This avoids non-reentrant log-cleanup
		// pause/resume (b/246804333).
		args = append(args, "-sysinfo=false")
	case "list":
		args = append(args, meta.ListFlags...)
	}
	args = append(args, meta.Target)
	args = append(args, patterns...)
	cmd := exec.CommandContext(ctx, meta.TastPath, args...)
	return cmd, nil
}

// Exec execs the tast command using supplied arguments.
// subcmd contains the subcommand to use, e.g. "list" or "run".
// flags contains subcommand-specific flags.
// patterns contains a list of patterns matching tests.
// stdout.txt and stderr.txt output files are written unconditionally.
//
// NOTE:
// Exec does not report failing tests. Either check the results manually (see
// ParseResultsJSON) or use the higher-level RunAndEvaluate instead of Exec.
func Exec(ctx context.Context, s *testing.State, subcmd string, flags, patterns []string) (stdout, stderr []byte, err error) {
	cmd, err := NewCommand(ctx, s, subcmd, flags, patterns)
	if err != nil {
		return nil, nil, err
	}

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

// RunAndEvaluate runs tests and checks the results, propagating any test failure. It returns (the names of) any skipped tests.
func RunAndEvaluate(ctx context.Context, s *testing.State, flags, patterns []string, resultsDir string, allowSkippedTests bool) []string {
	// Run tests.
	flags = append(flags, "-resultsdir="+resultsDir)
	if stdout, _, err := Exec(ctx, s, "run", flags, patterns); err != nil {
		lines := strings.Split(strings.TrimSpace(string(stdout)), "\n")
		s.Errorf("Failed to run tast: %v (last line: %q)", err, lines[len(lines)-1])
	}

	// Evaluate results.
	results, err := ParseResultsJSON(resultsDir)
	if err != nil {
		s.Error("Failed to parse test results: ", err)
	}
	var skippedTests []string
	for _, result := range results {
		if len(result.Errors) != 0 {
			s.Errorf("Test %s failed: %v", result.Name, result.Errors)
		}
		if result.SkipReason != "" {
			s.Logf("Test %s was skipped: %s", result.Name, result.SkipReason)
			skippedTests = append(skippedTests, result.Name)
		}
	}
	if len(skippedTests) > 0 && !allowSkippedTests {
		s.Errorf("Skipped %d test(s) in total: %v", len(skippedTests), skippedTests)
	}
	return skippedTests
}
