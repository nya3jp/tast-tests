// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package expectations provides tools for generating expectations file paths
// and a definition of a test expectation structure. An expectations file is
// used to modify test behavior, like documenting a known, triaged failing
// test as "expected to fail" or "skip". The file matches particular DUT types,
// via the model, build variant, board, or platform.
package expectations

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/crosconfig"
	"chromiumos/tast/lsbrelease"
	"chromiumos/tast/testing"
)

const expectationsDirectory = "/usr/local/graphics/expectations"

// FileType contains the DUT attribute that is matched when opening a test
// expectations file. Each type has a different naming convention.
type FileType string

const (
	// ModelFile files have the format "model-<DUT model>.yml"
	ModelFile FileType = "model"
	// BuildBoardFile files have the format "buildboard-<DUT build>.yml"
	BuildBoardFile FileType = "buildboard"
	// BoardFile files have the format "board-<DUT board>.yml". board is derived
	// from the DUT build variant and omits suffixes like "-kernelnext" or "64"
	BoardFile FileType = "board"
	// PlatformFile files have the format "platform-<DUT platform>.yml"
	PlatformFile FileType = "platform"
	// NoFile is used when there is not an expectations file for a test case
	NoFile FileType = "none"
)

const expectationsFileExtension = "yml"

// convertBuildToBoard returns the board string from a build string.
// ChromeOS build strings begin with the board type and possibly contain a
// suffix for the build variant. I.e. `-kernelnext` or `64`.
// Examples:
// | build string       | board string |
// |--------------------|--------------|
// | trogdor            | trogdor      |
// | trogdor-kernelnext | trogdor      |
// | trogdor64          | trogdor      |
func convertBuildToBoard(variant string) string {
	re := regexp.MustCompile(`^[a-zA-Z]+`)
	return re.FindString(variant)
}

// GenerateTestExpectationsFilenameWithDirectory generates a test expectations
// file name using the specified directory location. Probes the device model,
// board, or platform to generate the file name.
func GenerateTestExpectationsFilenameWithDirectory(ctx context.Context, testExpectationDirectory string, fileType FileType) (string, error) {
	var err error
	var name string

	switch fileType {
	case ModelFile:
		name, err = crosconfig.Get(ctx, "/", "name")
		if err != nil {
			return name, errors.Wrap(err, "unable to find model")
		}
	case BoardFile, BuildBoardFile:
		var ok bool
		lsbValues, err := lsbrelease.Load()
		if err != nil {
			return name, errors.Wrap(err, "failed to get lsb-release info")
		}
		name, ok = lsbValues[lsbrelease.Board]
		if !ok {
			return name, errors.New("unable to find board")
		}
		if fileType == BoardFile {
			name = convertBuildToBoard(name)
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

// getTestExpectationsDirectory generates the test expectations directory from
// a test name. For example, tast.video.PlatformDecoding will return the path
// /usr/local/graphics/expectations/tast/video/PlatformDecoding/.
func getTestExpectationsDirectory(testName string) string {
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

// GenerateTestExpectationsFilenameFromTestName generates a test expectations
// file name using the default directory location.
func GenerateTestExpectationsFilenameFromTestName(ctx context.Context, testName string, fileType FileType) (string, error) {
	return GenerateTestExpectationsFilenameWithDirectory(ctx, getTestExpectationsDirectory(testName), fileType)
}

// ReadTestExpectationsWithDirectory opens an existing test expectations file
// based on the device model, board, or platform. Looks in
// testExpectationsDirectory for test expectations files.
func ReadTestExpectationsWithDirectory(ctx context.Context, s *testing.State, testExpectationsDirectory string) ([]byte, FileType, error) {
	// Try the following file names:
	// 1. base_directory/model-<model>.yml
	// 2. base_directory/buildboard-<buildboard>.yml
	// 3. base_directory/board-<board>.yml
	// 4. base_directory/platform-<platform>.yml
	// The contents of the first of these files will be returned. If more
	// than one matching file exists, only the first will be used.
	for _, fileType := range []FileType{ModelFile, BuildBoardFile, BoardFile, PlatformFile} {
		filename, err := GenerateTestExpectationsFilenameWithDirectory(ctx, testExpectationsDirectory, fileType)
		if err != nil {
			return make([]byte, 0), NoFile, errors.Wrap(err, "failed to generate test expectations file name")
		}
		contents, err := os.ReadFile(filename)
		if err == nil {
			testing.ContextLogf(ctx, "Found %s-based expectations file at %s", fileType, filename)
			return contents, fileType, nil
		}
	}

	return make([]byte, 0), NoFile, errors.New("unable to find expectations file")
}

// ReadTestExpectations opens an existing test expectations file based on the
// device model, board, or platform. Uses the default directory naming scheme.
func ReadTestExpectations(ctx context.Context, s *testing.State) ([]byte, FileType, error) {
	return ReadTestExpectationsWithDirectory(ctx, s, getTestExpectationsDirectory(s.TestName()))
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
	// only be used for test cases that leave a DUT in invalid state.
	ExpectSkip Type = "SKIP"
)

// Expectation data describes the expected behavior for a particular test case.
// Ticket should be provided for readability and logging.
type Expectation struct {
	Expectation Type   `yaml:"expectation"`
	Ticket      string `yaml:"ticket"`
}
