// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package expectations provides tools for generating expectations file paths
// and a definition of a test expectation structure. An expectations file is
// used to modify test behavior, like documenting a known, triaged failing
// test as "expected to fail". The file matches particular DUT types,
// via the model, build variant, board, or gpu chipset.
package expectations

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strings"

	"gopkg.in/yaml.v2"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/crosconfig"
	"chromiumos/tast/lsbrelease"
	"chromiumos/tast/testing"
)

const expectationsDirectory = "/usr/local/graphics/expectations"

// debugLogging is a runtime variable to enable more debugging logs about the
// use of expectations files. When set to "true" to, it turns on log messages
// from the debugLog and debugLogf functions. Usage is:
//     tast run -var=expectations.DebugLogging=true <DUT> <test name pattern>
var debugLogging = testing.RegisterVarString(
	"expectations.DebugLogging",
	"false",
	"Set to true to enable debug logging for expectations. Example: --var=expectations.DebugLogging=true")

// debugLog will write to the context log when the verboseLogging runtime
// variable is set to true by a user. Use of debugLog is recommended for
// messages that assist with debugging the loading and parsing of expectations
// files.
func debugLog(ctx context.Context, args ...interface{}) {
	if debugLogging.Value() == "true" {
		testing.ContextLog(ctx, args...)
	}
}

// debugLogf will write to the context log when the verboseLogging runtime
// variable is set to true by a user. Use of debugLogf is recommended for
// messages that assist with debugging the loading and parsing of expectations
// files.
func debugLogf(ctx context.Context, format string, args ...interface{}) {
	if debugLogging.Value() == "true" {
		testing.ContextLogf(ctx, format, args...)
	}
}

// fileType contains the DUT attribute that is matched when opening a test
// expectations file. Each type has a different naming convention.
type fileType string

const (
	// modelFile files have the format "model-<DUT model>.yml"
	modelFile fileType = "model"
	// buildBoardFile files have the format "buildboard-<DUT build>.yml"
	buildBoardFile fileType = "buildboard"
	// boardFile files have the format "board-<DUT board>.yml". board is derived
	// from the DUT build variant and omits suffixes like "-kernelnext" or "64"
	boardFile fileType = "board"
	// gpuChipsetFile files have the format "chipset-<DUT GPU chipset>.yml"
	// where the chipset is determined by /usr/local/graphics/hardware_probe
	gpuChipsetFile fileType = "chipset"
	// all files will match for any DUT regardless of type.
	allDevicesFile fileType = "all"
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

// generateTestExpectationsFilename generates a test expectations
// file name using the specified directory location. Probes the device model,
// board, or gpu chipset to generate the file name.
func generateTestExpectationsFilename(ctx context.Context, testExpectationDirectory string, theFileType fileType) (string, error) {
	var err error
	var name string

	switch theFileType {
	case modelFile:
		name, err = crosconfig.Get(ctx, "/", "name")
		if err != nil {
			return name, errors.Wrap(err, "unable to find model")
		}
	case boardFile, buildBoardFile:
		var ok bool
		lsbValues, err := lsbrelease.Load()
		if err != nil {
			return name, errors.Wrap(err, "failed to get lsb-release info")
		}
		name, ok = lsbValues[lsbrelease.Board]
		if !ok {
			return name, errors.New("unable to find board")
		}
		if theFileType == boardFile {
			name = convertBuildToBoard(name)
		}
	case gpuChipsetFile:
		stdout, _, err := testexec.CommandContext(ctx, "/usr/local/graphics/hardware_probe").SeparatedOutput(testexec.DumpLogOnError)
		if err != nil {
			return name, errors.Wrap(err, "failed to get GPU chipset")
		}
		name = strings.TrimSpace(string(stdout))
	case allDevicesFile:
		return fmt.Sprintf("%s/%s.%s", testExpectationDirectory, theFileType, expectationsFileExtension), err
	default:
		return name, errors.Errorf("invalid expectation type: %s", theFileType)
	}
	return fmt.Sprintf("%s/%s-%s.%s", testExpectationDirectory, theFileType, name, expectationsFileExtension), err
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

// Type describes the expected test behavior.
type Type string

const (
	// ExpectPass is the default behavior - the decoder produces accurate
	// MD5 checksums and exits with code 0. This is only used internally.
	// Do not use this type within an expectations file.
	ExpectPass Type = "PASS"
	// ExpectFailure behavior is that the decoder will produce the wrong MD5
	// checksums or have non-zero exit code
	ExpectFailure Type = "FAIL"
)

// Expectation data describes the expected behavior for a particular test case.
// Ticket should be provided for readability and logging.
// Comments and SinceBuild are informational.
type Expectation struct {
	Expectation  Type            `yaml:"expectation"`
	Tickets      []string        `yaml:"tickets"` // I.e. [ "b/123", "crbug.com/456"]
	Comments     string          `yaml:"comments"`
	SinceBuild   string          `yaml:"since_build"` // I.e. "R107" or "R107-15144.0.0"
	ctx          context.Context // Used internally for context logging
	hasTastError bool            // Used to track the test error state without expectations being applied
}

// getExpectationYamlErrors returns nil if there are no errors with the YAML
// specification of the expectation. It checks the value of the Expectation
// field. This is only run on structures that have been successfully
// unmarshalled.
// It is assumed that expectations are primarily validated during preupload
// checks.
func getExpectationYamlErrors(e Expectation) error {
	// Validate the expectation type
	switch e.Expectation {
	case ExpectFailure:
		break
	case "":
		return errors.New("Expectations file must specify the expectation field")
	case ExpectPass:
		// PASS is only used internally and must not be used in an expectations file
		fallthrough
	default:
		testing.ContextLogf(e.ctx, "Invalid expectation value %s in expectations file", e.Expectation)
		return errors.New("Expectations file had an invalid value for the expectation field")
	}
	return nil
}

// GetTestExpectationFromDirectory opens an existing test expectations file
// based on the device model, board, or gpu chipset. Looks in
// testExpectationsDirectory for test expectations files.
func GetTestExpectationFromDirectory(ctx context.Context, s *testing.State, testExpectationsDirectory string) (Expectation, error) {
	expectPass := Expectation{ExpectPass, make([]string, 0), "", "", ctx, false}
	// Try the following file names:
	// 1. base_directory/model-<model>.yml
	// 2. base_directory/buildboard-<buildboard>.yml
	// 3. base_directory/board-<board>.yml
	// 4. base_directory/chipset-<gpu chipset>.yml
	// 5. base_directory/all.yml
	// The contents of the first of these files will be returned. If more
	// than one matching file exists, only the first will be used.
	for _, theFileType := range []fileType{modelFile, buildBoardFile, boardFile, gpuChipsetFile, allDevicesFile} {
		filename, err := generateTestExpectationsFilename(ctx, testExpectationsDirectory, theFileType)
		if err != nil {
			return expectPass, errors.Wrap(err, "failed to generate test expectations file name")
		}
		debugLogf(ctx, "Looking for %s expectations file %s", theFileType, filename)
		contents, err := os.ReadFile(filename)
		if err == nil {
			testing.ContextLogf(ctx, "Found %s expectations file at %s", theFileType, filename)
			var expectation Expectation
			if s.Param() != nil {
				// The test is parameterized. I.e. tast.<package>.<test name>.<test case>
				// For parameterized tests, the YAML structure contains a map of the test
				// name to an expectation. For example:
				//
				// <package>.<test name>.<test case>:
				//   expectation: FAIL
				//   tickets:
				//   - "b/12345"
				//   - "crbug/67890"
				//   comments: "The test has an expectation for the following reason: ..."
				//   sinceBuild: "R100-14526.89.0"
				//
				// If there is no key for the test, then it is expected to pass.
				expectations := make(map[string]Expectation)
				err = yaml.Unmarshal(contents, &expectations)
				if err != nil {
					return expectPass, errors.Wrap(err, "unable to parse expectations file")
				}
				var ok bool
				expectation, ok = expectations[s.TestName()]
				if !ok {
					return expectPass, nil
				}
			} else {
				// The test is not parameterized. I.e. tast.<package>.<test name>
				// The file contains only one Expectation.
				var expectation Expectation
				err = yaml.Unmarshal(contents, expectation)
				if err != nil {
					return expectPass, errors.Wrap(err, "unable to parse expectations file")
				}
			}
			expectation.ctx = ctx
			return expectation, getExpectationYamlErrors(expectation)
		}
	}

	return expectPass, nil
}

// GetTestExpectation opens an existing test expectations file based on the
// device model, board, or gpu chipset. Uses the default directory naming scheme.
func GetTestExpectation(ctx context.Context, s *testing.State) (Expectation, error) {
	return GetTestExpectationFromDirectory(ctx, s, getTestExpectationsDirectory(s.TestName()))
}

// Error is an expectation aware wrapper over testing.State.Error. If the
// test is expected to fail, then the error is demoted to a log.
func (e *Expectation) Error(s *testing.State, args ...interface{}) {
	e.hasTastError = true
	switch e.Expectation {
	case ExpectPass:
		s.Error(args...)
	case ExpectFailure:
		testing.ContextLog(e.ctx, append([]interface{}{"Error:"}, args...))
	}
}

// Errorf is an expectation aware wrapper over testing.State.Errorf. If the
// test is expected to fail, then the error is demoted to a log.
func (e *Expectation) Errorf(s *testing.State, format string, args ...interface{}) {
	e.hasTastError = true
	switch e.Expectation {
	case ExpectPass:
		s.Errorf(format, args...)
	case ExpectFailure:
		testing.ContextLogf(e.ctx, "Error: "+format, args...)
	}
}

// Fatal is an expectation aware wrapper over testing.State.Fatal. If the
// test is expected to fail, then the fatal error is demoted to a log, but
// the test still stops.
func (e *Expectation) Fatal(s *testing.State, args ...interface{}) {
	e.hasTastError = true
	switch e.Expectation {
	case ExpectPass:
		s.Fatal(args...)
	case ExpectFailure:
		testing.ContextLog(e.ctx, append([]interface{}{"Fatal:"}, args...))
		runtime.Goexit()
	}
}

// Fatalf is an expectation aware wrapper over testing.State.Fatalf. If the
// test is expected to fail, then the fatal error is demoted to a log, but
// the test still stops.
func (e *Expectation) Fatalf(s *testing.State, format string, args ...interface{}) {
	e.hasTastError = true
	switch e.Expectation {
	case ExpectPass:
		s.Fatalf(format, args...)
	case ExpectFailure:
		testing.ContextLogf(e.ctx, "Fatal: "+format, args...)
		runtime.Goexit()
	}
}

// HandleFinalExpectation will cause the test case to fail if there was no error,
// but the expectation was to fail. Calling this should be deferred by a test.
func (e *Expectation) HandleFinalExpectation(s *testing.State) {
	if e.Expectation == ExpectFailure && !e.hasTastError {
		var ticketsMessage string
		if len(e.Tickets) > 0 {
			ticketsMessage = " due to " + strings.Join(e.Tickets, ", ")
		}
		s.Errorf("Test passed! Consider removing %s expectation%s", e.Expectation, ticketsMessage)
	} else if e.Expectation == ExpectFailure && e.hasTastError {
		var sinceBuildMessage string
		if len(e.SinceBuild) > 0 {
			sinceBuildMessage = " since " + e.SinceBuild
		}
		var ticketsMessage string
		if len(e.Tickets) > 0 {
			ticketsMessage = " due to " + strings.Join(e.Tickets, ", ")
		}
		var commentsMessage string
		if len(e.Comments) > 0 {
			commentsMessage = " (" + e.Comments + ")"
		}

		testing.ContextLogf(e.ctx, "The test encountered Tast errors. These were ignored due to existing expectation%s%s%s", sinceBuildMessage, ticketsMessage, commentsMessage)
	}
}
