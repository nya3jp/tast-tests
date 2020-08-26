// Copyright 2020 The Chromium OS Authors. All rights reserved.
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
	"reflect"
	"strconv"
	"strings"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// Run execs the tast command using supplied arguments.
// subcmd contains the subcommand to use, e.g. "list" or "run".
// flags contains subcommand-specific flags.
// patterns contains a list of patterns matching tests.
// stdout.txt and stderr.txt output files are written unconditionally.
func Run(ctx context.Context, s *testing.State, subcmd string, flags, patterns []string) (stdout, stderr []byte, err error) {
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

// RunTestWithFlags runs tets with args given flags
// -resultsdir flag will be added to the parent test's outdir.
func RunTestWithFlags(ctx context.Context, s *testing.State, flags []string, testName string) error {
	num, err := getFileNum(s.OutDir(), "subtest_result")
	resultsDir := filepath.Join(s.OutDir(), "subtest_result_"+strconv.Itoa(num+1)+"_"+testName)
	flags = append(flags, "-resultsdir="+resultsDir)
	// These are subsets of the testing.Error and TestResult structs.
	type testError struct {
		Reason string `json:"reason"`
	}
	type testResult struct {
		Name   string      `json:"name"`
		Errors []testError `json:"errors"`
	}
	s.Log("Run test: ", testName)
	stdout, stderr, err := Run(ctx, s, "run", flags, []string{testName})
	s.Log("stdout: ", string(stdout))
	if err != nil {
		s.Log("Run Error: ", err)
		s.Log("stderr: ", string(stderr))
		lines := strings.Split(strings.TrimSpace(string(stdout)), "\n")
		return mtbferrors.New(mtbferrors.ARCRunTast, err, testName, lines[len(lines)-1])
	}

	var results []testResult
	rf, err := os.Open(filepath.Join(resultsDir, "results.json"))
	if err != nil {
		return mtbferrors.New(mtbferrors.ARCOpenResult, err)
	}
	defer rf.Close()

	if err = json.NewDecoder(rf).Decode(&results); err != nil {
		return mtbferrors.New(mtbferrors.ARCParseResult, err, rf.Name())
	}

	expResults := []testResult{
		testResult{testName, nil},
	}
	if !reflect.DeepEqual(results, expResults) {
		return errors.Errorf("%+v , failed in subcase: %v", results[0].Errors[0].Reason, testName)
	}
	return nil
}

func getFileNum(path, prefix string) (int, error) {
	i := 0
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return 0, err
	}
	for _, file := range files {
		fileName := file.Name()
		if strings.HasPrefix(fileName, prefix) {
			i++
		}
	}
	return i, nil
}
