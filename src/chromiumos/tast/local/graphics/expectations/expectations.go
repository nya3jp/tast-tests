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
	"strconv"
	"strings"

	"gopkg.in/yaml.v2"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/crosconfig"
	"chromiumos/tast/local/graphics"
	"chromiumos/tast/lsbrelease"
	"chromiumos/tast/testing"
)

const expectationsDirectory = "/usr/local/graphics/expectations"

// disable is a runtime variable to disable loading expectations when "true"
var disable = testing.RegisterVarString(
	"expectations.disable",
	"false",
	"Set to true to disable expectations usage. Example: --var=expectations.disable=true")

// isDisabled returns true when the runtime variable expectations.Disable has
// been set to "true".
func isDisabled() (bool, error) {
	value, err := strconv.ParseBool(disable.Value())
	if err != nil {
		return false, errors.Wrap(err, "failed to parse runtime variable "+disable.Name())
	}
	return value, nil
}

// debugLogging is a runtime variable to enable more debugging logs about the
// use of expectations files. When set to "true" to, it turns on log messages
// from the debugLog and debugLogf functions. Usage is:
//
//	tast run -var=expectations.DebugLogging=true <DUT> <test name pattern>
var debugLogging = testing.RegisterVarString(
	"expectations.debugLogging",
	"false",
	"Set to true to enable debug logging for expectations. Example: --var=expectations.debugLogging=true")

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

// overrideModel can be specified to override the model identity. This can be
// used for manual testing and for early bringup.
var overrideModel = testing.RegisterVarString(
	"expectations.overrideModel",
	"",
	"Set to override the detected model. Example: --var=expectations.overrideModel=baz")

// overrideBuildBoard can be specified to override the "build board" identity.
// This can be used for manual testing and for early bringup.
var overrideBuildBoard = testing.RegisterVarString(
	"expectations.overrideBuildBoard",
	"",
	"Set to override the detected build board. Example: --var=expectations.overrideBuildBoard=baz")

// overrideBoard can be specified to override the board identity.
// This can be used for manual testing and for early bringup.
var overrideBoard = testing.RegisterVarString(
	"expectations.overrideBoard",
	"",
	"Set to override the detected board. Example: --var=expectations.overrideBoard=baz")

// overrideChipset can be specified to override the GPU chipset identity.
// This can be used for manual testing and for early bringup.
var overrideChipset = testing.RegisterVarString(
	"expectations.overrideChipset",
	"",
	"Set to override the detected chipset. Example: --var=expectations.overrideChipset=baz")

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

// getDeviceModel returns the model of the running device.
func getDeviceModel(ctx context.Context) (string, error) {
	if len(overrideModel.Value()) > 0 {
		testing.ContextLog(ctx, "Device model has been overriden to ", overrideModel.Value())
		return overrideModel.Value(), nil
	}

	model, err := crosconfig.Get(ctx, "/", "name")
	if err != nil {
		// Fallback to trying crossystem
		debugLog(ctx, "Failed to determine the model from crosconfig. Trying crossystem hwid.")

		out, errCrossystem := testexec.CommandContext(ctx, "crossystem", "hwid").Output()
		if errCrossystem != nil {
			return "", errors.Wrap(errCrossystem, "unable to find model")
		}
		hwid := string(out)

		// hwid is in the form of: '<MODEL>-AAAA ADA-ADA-ADA-ADA-ADA' or
		// '<MODEL> ADA-ADA-ADA-ADA-ADA' where A is upper case
		// alphabetic, and D is a digit.
		modelRe := regexp.MustCompile(`^[A-Z]*`) // matches the model part of an hwid

		upperCaseModel := modelRe.FindString(hwid)
		if len(upperCaseModel) == 0 {
			return "", errors.Wrap(err, "unable to find model")
		}
		model = strings.ToLower(upperCaseModel)
	}

	return model, nil
}

// getDeviceBuildBoardWithoutOverride gets the board and build from the running
// device lsbrelease. Ignores overrideBuildBoard runtime variable.
func getDeviceBuildBoardWithoutOverride(ctx context.Context) (string, error) {
	lsbValues, err := lsbrelease.Load()
	if err != nil {
		return "", errors.Wrap(err, "failed to get lsb-release info")
	}

	buildBoard, ok := lsbValues[lsbrelease.Board]
	if !ok {
		return "", errors.New("unable to find board")
	}

	return buildBoard, nil
}

// getDeviceBuildBoard gets the board and build from the running device lsbrelease.
func getDeviceBuildBoard(ctx context.Context) (string, error) {
	if len(overrideBuildBoard.Value()) > 0 {
		testing.ContextLog(ctx, "Device build board has been overriden to ", overrideBuildBoard.Value())
		return overrideBuildBoard.Value(), nil
	}

	return getDeviceBuildBoardWithoutOverride(ctx)
}

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

// getDeviceBuild gets the board from the running device lsbrelease.
func getDeviceBuild(ctx context.Context) (string, error) {
	if len(overrideBoard.Value()) > 0 {
		testing.ContextLog(ctx, "Device board has been overriden to ", overrideBoard.Value())
		return overrideBoard.Value(), nil
	}

	buildBoard, err := getDeviceBuildBoardWithoutOverride(ctx)
	if err != nil {
		return "", err
	}

	return convertBuildToBoard(buildBoard), nil
}

// getDeviceChipset gets the GPU chipset ID from the running device.
func getDeviceChipset(ctx context.Context) (string, error) {
	if len(overrideChipset.Value()) > 0 {
		testing.ContextLog(ctx, "Device GPU chipset has been overriden to ", overrideChipset.Value())
		return overrideChipset.Value(), nil
	}

	gpu, err := graphics.GPUFamilies(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to get GPU chipset")
	}
	// We use the first GPU found for expectation.
	return gpu[0], nil
}

// getDeviceIdentifier returns an identifier for the running device based on fileType.
func (ft *fileType) getDeviceIdentifier(ctx context.Context) (string, error) {
	if *ft == allDevicesFile {
		return string(*ft), nil
	}

	var id string
	var err error
	switch *ft {
	case modelFile:
		id, err = getDeviceModel(ctx)
	case buildBoardFile:
		id, err = getDeviceBuildBoard(ctx)
	case boardFile:
		id, err = getDeviceBuild(ctx)
	case gpuChipsetFile:
		id, err = getDeviceChipset(ctx)
	default:
		return "", errors.Errorf("invalid identifier type: %s", *ft)
	}

	return fmt.Sprintf("%s-%s", *ft, id), err
}

// logDeviceIdentity creates a context log with the identity that will be used
// for loading expectations files.
func logDeviceIdentity(ctx context.Context) {
	var err error
	fileTypes := []fileType{modelFile, buildBoardFile, boardFile, gpuChipsetFile}
	identifiers := make([]string, len(fileTypes))
	for idx, ft := range fileTypes {
		identifiers[idx], err = ft.getDeviceIdentifier(ctx)
		if err != nil {
			identifiers[idx] = fmt.Sprintf("%s-unknown", ft)
		}
	}

	testing.ContextLog(ctx, "Device has the following expectations identity: ", strings.Join(identifiers, ", "))
}

// generateTestExpectationsFilename generates a test expectations file name
// using the specified directory location. Depending on the fileType, this may
// probe the device model, board, or gpu chipset to generate the file name.
func generateTestExpectationsFilename(ctx context.Context, testExpectationDirectory string, ft fileType) (string, error) {
	identifier, err := ft.getDeviceIdentifier(ctx)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/%s.%s", testExpectationDirectory, identifier, expectationsFileExtension), nil
}

// getTestExpectationsDirectory generates the test expectations directory from
// a test name. For example, tast.video.PlatformDecoding will return the path
// /usr/local/graphics/expectations/tast/video/PlatformDecoding/.
func getTestExpectationsDirectory(testName string) (string, error) {
	// Tast test names should never more than 3 deep:
	// <test package>.<test name> or
	// <test package>.<test name>.<test case>
	// Use only the test package and test name, not the test case to build
	// the test expectations directory name.

	// NOTE: if the Tast adds levels to test names in the future, then this
	// function will need to be updated. The function returns an error if
	// the test name has too many or too few levels.
	testNameSlice := strings.Split(testName, ".")
	switch {
	case len(testNameSlice) < 2:
		return "", errors.Errorf("test name %s should contain at least the package and test function", testName)
	case len(testNameSlice) == 2:
		fallthrough
	case len(testNameSlice) == 3:
		// Test name is formatted expectedly
		return fmt.Sprintf("%s/tast/%s/%s", expectationsDirectory, testNameSlice[0], testNameSlice[1]), nil
	default:
		return "", errors.Errorf("test name %s contains more than two periods", testName)
	}
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
	Tickets      []string        `yaml:"tickets,omitempty"` // I.e. [ "b/123", "crbug.com/456"]
	Comments     string          `yaml:"comments,omitempty"`
	SinceBuild   string          `yaml:"since_build,omitempty"` // I.e. "R107" or "R107-15144.0.0"
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

// expectPass creates a "passing" expectation for use when no expectation is
// found for the test case.
func expectPass(ctx context.Context) Expectation {
	return Expectation{ExpectPass, make([]string, 0), "", "", ctx, false}
}

// GetTestExpectationFromDirectory opens an existing test expectations file
// based on the device model, board, or gpu chipset. Looks in
// testExpectationsDirectory for test expectations files.
func GetTestExpectationFromDirectory(ctx context.Context, testName, testExpectationsDirectory string) (Expectation, error) {
	logDeviceIdentity(ctx)

	disabled, err := isDisabled()
	if err != nil {
		return expectPass(ctx), err
	}
	if disabled {
		testing.ContextLogf(ctx, "Loading expectations was disabled to runtime variable %s=%s", disable.Name(), disable.Value())
		return expectPass(ctx), nil
	}

	// Try the following file names:
	// 1. base_directory/model-<model>.yml
	// 2. base_directory/buildboard-<buildboard>.yml
	// 3. base_directory/board-<board>.yml
	// 4. base_directory/chipset-<gpu chipset>.yml
	// 5. base_directory/all.yml
	// The contents of the first of these files will be returned. If more
	// than one matching file exists, only the first will be used.
	for _, ft := range []fileType{modelFile, buildBoardFile, boardFile, gpuChipsetFile, allDevicesFile} {
		filename, err := generateTestExpectationsFilename(ctx, testExpectationsDirectory, ft)
		if err != nil {
			return expectPass(ctx), errors.Wrap(err, "failed to generate test expectations file name")
		}
		debugLogf(ctx, "Looking for %s expectations file %s", ft, filename)
		contents, err := os.ReadFile(filename)
		if err == nil {
			testing.ContextLogf(ctx, "Found %s expectations file at %s", ft, filename)
			// The YAML structure contains a map of the test name to an expectation. For
			// parameterized tests, each test case can have its own expectation. For
			// example:
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
			// For non-parameterized test cases, there will be only one key:
			// "<package>.<test name>".
			expectations := make(map[string]Expectation)
			err = yaml.Unmarshal(contents, &expectations)
			if err != nil {
				return expectPass(ctx), errors.Wrap(err, "unable to parse expectations file")
			}
			expectation, ok := expectations[testName]
			if !ok {
				return expectPass(ctx), nil
			}
			expectation.ctx = ctx
			return expectation, getExpectationYamlErrors(expectation)
		}
	}

	return expectPass(ctx), nil
}

// GetTestExpectation opens an existing test expectations file based on the
// device model, board, or gpu chipset. Uses the default directory naming scheme.
func GetTestExpectation(ctx context.Context, testName string) (Expectation, error) {
	directory, err := getTestExpectationsDirectory(testName)
	if err != nil {
		return expectPass(ctx), err
	}
	return GetTestExpectationFromDirectory(ctx, testName, directory)
}

// ReportError is used to get the preferred error handling within the
// context of a test expectation. If the return value is not nil, then
// the test code should use the error as an input to Error or Fatal.
// If the test code must not continue after the error, it is up to the
// caller to guarantee to stop the test.
func (e *Expectation) ReportError(args ...interface{}) error {
	e.hasTastError = true
	switch e.Expectation {
	case ExpectPass:
		return errors.New(fmt.Sprint(args...))
	case ExpectFailure:
		testing.ContextLog(e.ctx, append([]interface{}{"Error:"}, args...))
	}
	return nil
}

// ReportErrorf is used to get the preferred error handling within the
// context of a test expectation. If the return value is not nil, then
// the test code should use the error as an input to Error or Fatal.
// If the test code must not continue after the error, it is up to the
// caller to guarantee to stop the test.
func (e *Expectation) ReportErrorf(format string, args ...interface{}) error {
	e.hasTastError = true
	switch e.Expectation {
	case ExpectPass:
		return errors.Errorf(format, args...)
	case ExpectFailure:
		testing.ContextLogf(e.ctx, "Error: "+format, args...)
	}
	return nil
}

// HandleFinalExpectation will cause the test case to fail if there was no error,
// but the expectation was to fail. Calling this should be deferred by a test.
func (e *Expectation) HandleFinalExpectation() error {
	if e.Expectation == ExpectFailure && !e.hasTastError {
		var ticketsMessage string
		if len(e.Tickets) > 0 {
			ticketsMessage = " due to " + strings.Join(e.Tickets, ", ")
		}
		return errors.Errorf("Test passed! Consider removing %s expectation%s", e.Expectation, ticketsMessage)
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
	return nil
}
