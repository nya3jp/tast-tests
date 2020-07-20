// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ml contains helper functions that are used
// when executing ML-related Tast tests
package ml

import (
	"fmt"
	"os"
	"path/filepath"

	"chromiumos/tast/testing"
)

// ContainsAll checks that sliceToQuery is a superset of sliceToMatch.
func ContainsAll(sliceToQuery, sliceToMatch []string) bool {
	for _, item := range sliceToMatch {
		if !contains(sliceToQuery, item) {
			return false
		}
	}
	return true
}

// contains checks that sliceToQuery contains an instance of toFind.
func contains(sliceToQuery []string, toFind string) bool {
	for _, item := range sliceToQuery {
		if item == toFind {
			return true
		}
	}
	return false
}

// LogOutput will write out the parameters to logFilename
func LogOutput(s *testing.State, logFilename, cmd, stdout, stderr string) {
	logf, err := os.Create(filepath.Join(s.OutDir(), logFilename))
	if err != nil {
		s.Fatal("Failed to create logfile: ", err)
	}
	defer logf.Close()

	fmt.Fprintln(logf, cmd)
	fmt.Fprintln(logf, "stdout:")
	fmt.Fprintln(logf, stdout)
	fmt.Fprintln(logf, "stderr:")
	fmt.Fprintln(logf, stderr)
}
