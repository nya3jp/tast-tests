// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package expectations provides tools for generating expectations file paths
// and a definition of a test expectation structure.
package expectations

import (
	"context"
	"fmt"
	"os"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/crosconfig"
	"chromiumos/tast/lsbrelease"
	"chromiumos/tast/testing"
)

const expectationsDirectory = "/usr/local/graphics/expectations"

// GetTestExpectationsDirectory generates the test expectations directory from a test name
func GetTestExpectationsDirectory(testName string) string {
	// Tast test names are never more than 3 deep:
	// <test package>.<test name> or
	// <test package>.<test name>.<test case>
	// Use only the test package and test name, not the test case to build
	// the test expectations directory name.
	testNameSlice := strings.Split(testName, ".")

	directory := fmt.Sprintf("%s/tast", expectationsDirectory)
	if len(testNameSlice) > 0 {
		directory += "/" + testNameSlice[0]
	}
	if len(testNameSlice) > 1 {
		directory += "/" + testNameSlice[1]
	}
	return directory
}

// FileType contains the DUT attribute that is matched when opening a test
// expectations file. Each type has a different naming convention.
type FileType string

const (
	// ModelFile files have the format "model-<DUT model>.yml"
	ModelFile FileType = "model"
	// BoardFile files have the format "board-<DUT board>.yml"
	BoardFile FileType = "board"
	// PlatformFile files have the format "platform-<DUT platform>.yml"
	PlatformFile FileType = "platform"
	// NoFile is used when there is not an expectations file for a test case
	NoFile FileType = "none"
)

const expectationsFileExtension = "yml"

// GenerateTestExpectationsFilenameWithDirectory generates a test expectations
// file name using the specified directory location. Probes the device model,
// board, or platfom to generate the file name.
func GenerateTestExpectationsFilenameWithDirectory(ctx context.Context, testExpectationDirectory string, fileType FileType) (string, error) {
	var err error
	var name string

	switch fileType {
	case ModelFile:
		name, err = crosconfig.Get(ctx, "/", "name")
		if err != nil {
			return name, errors.Wrap(err, "unable to find model")
		}
	case BoardFile:
		var ok bool
		lsbValues, err := lsbrelease.Load()
		if err != nil {
			return name, errors.Wrap(err, "failed to get lsb-release info")
		}
		name, ok = lsbValues[lsbrelease.Board]
		if !ok {
			return name, errors.New("unable to find board")
		}
	case PlatformFile:
		name, err = crosconfig.Get(ctx, "/identity", "platform-name")
		if err != nil {
			return name, errors.Wrap(err, "unable to find platform")
		}
	default:
		return name, errors.Errorf("invalid expectation type: %s", fileType)
	}
	return fmt.Sprintf("%s/%s-%s.%s", testExpectationDirectory, fileType, name, expectationsFileExtension), err
}

// GenerateTestExpectationsFilenameFromTestName generates a test expectations file name using the default directory location.
func GenerateTestExpectationsFilenameFromTestName(ctx context.Context, testName string, fileType FileType) (string, error) {
	return GenerateTestExpectationsFilenameWithDirectory(ctx, GetTestExpectationsDirectory(testName), fileType)
}

// OpenTestExpectationsWithDirectory opens an existing test expectations file
// based on the device model, board, or platform. Looks in
// testExpectationsDirectory for test expectations files.
func OpenTestExpectationsWithDirectory(ctx context.Context, s *testing.State, testExpectationsDirectory string) (*os.File, FileType, error) {
	var err error
	var f *os.File
	var filename string

	// Try the following file names:
	// base_directory/model-<model>.yml
	// base_directory/board-<board>.yml
	// base_directory/platform-<platform>.yml
	for _, fileType := range []FileType{ModelFile, BoardFile, PlatformFile} {
		filename, err = GenerateTestExpectationsFilenameWithDirectory(ctx, testExpectationsDirectory, fileType)
		if err != nil {
			s.Fatalf("Failed to determine device %s:", fileType)
			return f, NoFile, err
		}
		testing.ContextLogf(ctx, "Looking up expectations file at %s", filename)
		f, err = os.Open(filename)
		if err == nil {
			testing.ContextLogf(ctx, "Found %s-based expectations file", fileType)
			return f, fileType, err
		}
	}

	testing.ContextLog(ctx, "Unable to find expectations file")
	return nil, NoFile, errors.New("unable to find expectations file")
}

// OpenTestExpectations opens an existing test expectations file based on the
// device model, board, or platform. Uses the default directory naming scheme.
func OpenTestExpectations(ctx context.Context, s *testing.State) (*os.File, FileType, error) {
	return OpenTestExpectationsWithDirectory(ctx, s, GetTestExpectationsDirectory(s.TestName()))
}

// Type describes the expected test behavior.
type Type string

const (
	// ExpectPass is the default behavior - the decoder produces accurate
	// MD5 checksums and exits with code 0.
	ExpectPass Type = "PASS"
	// ExpectFailure behavior is that the decoder will produce the wrong MD5
	// checksums or have non-zero exit code
	ExpectFailure Type = "FAILURE"
	// ExpectSkip behavior tells the test to skip a test case. This should
	// be used for test cases that leave a DUT in invalid state.
	ExpectSkip Type = "SKIP"
)

// Expectation data describes the expected behavior for a particular test case.
// Ticket can be used for improved tracking and logging.
type Expectation struct {
	Expectation Type   `yaml:"expectation"`
	Ticket      string `yaml:"ticket"`
}
