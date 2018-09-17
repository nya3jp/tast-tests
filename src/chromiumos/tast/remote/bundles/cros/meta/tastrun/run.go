// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package tastrun helps meta tests run the tast command.
package tastrun

import (
	"errors"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strings"

	"chromiumos/tast/testing"
)

// Run execs the tast command using supplied arguments.
// subcmd contains the subcommand to use, e.g. "list" or "run".
// flags contains subcommand-specific flags.
// patterns contains a list of patterns matching tests.
// stdout is saved to an stdout.txt output file unconditionally.
// stderr is saved to stderr.txt if the command fails.
func Run(s *testing.State, subcmd string, flags, patterns []string) (stdout []byte, err error) {
	meta := s.Meta()
	if meta == nil {
		return nil, errors.New("failed to get meta info from context")
	}

	args := append([]string{subcmd}, flags...)
	args = append(args, meta.RunFlags...)
	args = append(args, meta.Target)
	args = append(args, patterns...)
	cmd := exec.CommandContext(s.Context(), meta.TastPath, args...)

	s.Log("Running ", strings.Join(cmd.Args, " "))
	out, err := cmd.Output()

	if werr := ioutil.WriteFile(filepath.Join(s.OutDir(), "stdout.txt"), out, 0644); werr != nil {
		s.Log("Failed to save stdout: ", werr)
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.Stderr != nil {
		if werr := ioutil.WriteFile(filepath.Join(s.OutDir(), "stderr.txt"), exitErr.Stderr, 0644); werr != nil {
			s.Log("Failed to save stderr: ", werr)
		}
	}
	return out, err
}
