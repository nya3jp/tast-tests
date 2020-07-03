// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package croshealthd provides methods for running and obtaining output from
// cros_healthd commands.
package croshealthd

import (
	"context"
	"encoding/csv"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/crosconfig"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
)

// TelemCategory represents a category flag that can be passed to the
// cros_healthd telem command.
type TelemCategory string

// Categories for the cros_healthd telem command.
const (
	TelemCategoryBacklight         TelemCategory = "backlight"
	TelemCategoryBattery           TelemCategory = "battery"
	TelemCategoryBluetooth         TelemCategory = "bluetooth"
	TelemCategoryCPU               TelemCategory = "cpu"
	TelemCategoryFan               TelemCategory = "fan"
	TelemCategoryMemory            TelemCategory = "memory"
	TelemCategoryStatefulPartition TelemCategory = "stateful_partition"
	TelemCategoryStorage           TelemCategory = "storage"
	TelemCategorySystem            TelemCategory = "system"
	TelemCategoryTimezone          TelemCategory = "timezone"
)

// RunTelem runs cros-health-tool's telem command with the given category and
// returns the output. It also dumps the output to a file for debugging. An
// error is returned if there is a failure to run the command or save the output
// to a file.
func RunTelem(ctx context.Context, category TelemCategory, outDir string) ([]byte, error) {
	if err := upstart.EnsureJobRunning(ctx, "cros_healthd"); err != nil {
		return nil, errors.Wrap(err, "failed to start cros_healthd")
	}

	args := []string{"telem", fmt.Sprintf("--category=%s", category)}
	b, err := testexec.CommandContext(ctx, "cros-health-tool", args...).Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "command failed")
	}

	// Log output to file for debugging.
	path := filepath.Join(outDir, "command_output.txt")
	if err := ioutil.WriteFile(path, b, 0644); err != nil {
		return nil, errors.Wrapf(err, "failed to write output to %s", path)
	}

	return b, nil
}

// RunAndParseTelem runs RunTelem and parses the CSV output into a
// two-dimensional array. An error is returned if there is a failure to obtain
// or parse the output or if a line of output has an unexpected number of
// fields.
func RunAndParseTelem(ctx context.Context, category TelemCategory, outDir string) ([][]string, error) {
	b, err := RunTelem(ctx, category, outDir)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run telem command")
	}

	records, err := csv.NewReader(strings.NewReader(string(b))).ReadAll()
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse output")
	}

	return records, nil
}

// ValidCSV is responsible for validating headers and all values of the CSV
// output. An error is returned if the headers are invalid or if any of the CSV
// values are incorrect as determined by the validators provided to verify a
// particular CSV value.
func ValidCSV(csv [][]string, headers []string, columns ...func(string) error) error {
	// Validate headers
	if ok := reflect.DeepEqual(csv[0], headers); !ok {
		return errors.Errorf("incorrect headers: got %v want %v", csv[0], headers)
	}
	// Validate columns
	for i, column := range columns {
		if err := column(csv[1][i]); err != nil {
			return errors.Wrapf(err, "failed validation on %v (Column %v) with value %v", headers[i], i, csv[1][i])
		}
	}
	return nil
}

// Headers returns a slice of the headers.
func Headers(headers ...string) []string {
	return headers
}

// Column takes in a list of validators that will verify different properties
// of |csvValue|. Normally, |validators| will have a size of 1 here which will
// be a top level validator (e.g. String, UInt64) that takes as input a list of
// more specific validators. Column returns a function that will be invoked in
// ValidCSV().
func Column(validators ...func(interface{}) error) func(string) error {
	return func(csvValue string) error {
		for _, validator := range validators {
			if err := validator(csvValue); err != nil {
				return err
			}
		}
		return nil
	}
}

// String returns a function that checks whether |actualValue| is of type string
// and invokes any validators that further verify the properties of
// |actualValue|.
func String(validators ...func(interface{}) error) func(interface{}) error {
	return func(actualValue interface{}) error {
		if str, ok := actualValue.(string); ok {
			// Invoke all the validators
			for _, validator := range validators {
				if err := validator(str); err != nil {
					return err
				}
			}
			return nil
		}
		return errors.Errorf("invalid parameter type in String: got %T, want string", actualValue)
	}
}

// UInt64 returns a function that checks whether |actualValue| is of type uint64 and
// invokes any validators that further verify the properties of |actualValue|.
func UInt64(validators ...func(interface{}) error) func(interface{}) error {
	return func(actualValue interface{}) error {
		if str, ok := actualValue.(string); ok {
			// Confirm |actualValue| type can be converted to uint64
			_, err := strconv.ParseUint(str, 10, 64)
			if err != nil {
				return errors.Wrapf(err, "failed to convert %v to uint64", str)
			}
			// Invoke all the validators
			for _, validator := range validators {
				if err = validator(str); err != nil {
					return err
				}
			}
			return nil
		}
		return errors.Errorf("invalid parameter type in UInt64: got %T, want string", actualValue)
	}
}

// CorrectFormat returns a function that checks whether |actualValue| matches
// the regex pattern specified by |regex|.
func CorrectFormat(regex interface{}) func(interface{}) error {
	return func(actualValue interface{}) error {
		var re *regexp.Regexp
		if r, ok := regex.(*regexp.Regexp); ok {
			re = r
		} else {
			return errors.Errorf("invalid parameter type in CorrectFormat: got %T, want *regexp.Regexp", regex)
		}
		if str, ok := actualValue.(string); ok {
			// If the value does not exist, do not check the format.
			if str == "NA" {
				return nil
			}
			if matched := re.MatchString(str); !matched {
				return errors.Errorf("failed to follow correct pattern: got %v, want %v", re, str)
			}
			return nil
		}
		return errors.Errorf("invalid parameter type in CorrectFormat: got %T, want string", actualValue)
	}
}

// EqualToFileContent returns a function that checks whether |filePathAndName|
// and exists. If it does, it compares the value at that location wih
// |actualValue|. If it does not, it ensures that |actualValue| equals "NA".
func EqualToFileContent(filePathAndName interface{}) func(interface{}) error {
	return func(actualValue interface{}) error {
		var file string
		if str, ok := filePathAndName.(string); ok {
			file = str
		} else {
			return errors.Errorf("invalid parameter type in EqualToFileContent: got %T, want string", filePathAndName)
		}
		if actual, ok := actualValue.(string); ok {
			expectedValueByteArr, err := ioutil.ReadFile(file)
			if os.IsNotExist(err) {
				if actual != "NA" {
					return errors.Errorf("failed to get correct value: got %v, want NA", actual)
				}
			} else if err != nil {
				return errors.Wrapf(err, "failed to read from file %v", file)
			} else {
				expected := strings.TrimRight(string(expectedValueByteArr), "\n")
				if actual != expected {
					return errors.Errorf("invalid parameter type in EqualToFileContent: got %v, want %v", actual, expected)
				}
			}
			return nil
		}
		return errors.Errorf("invalid parameter type in EqualToFileContent: got %T, want string", actualValue)
	}
}

// CheckFileContentIfFileShouldExist returns a function that checks whether
// |filePathAndName| should exist by using crosconfig and its two arguments,
// |crosConfigProperty| and |crosConfigPath|. If the file should exist, it
// attempts to read the value from the file. If it cannot, an error is
// reported. If it can, it compares the read value with |actualValue|.
func CheckFileContentIfFileShouldExist(ctx context.Context, crosConfigPath,
	crosConfigProperty, filePathAndName string) func(interface{}) error {
	return func(actualValue interface{}) error {
		val, err := crosconfig.Get(ctx, crosConfigPath, crosConfigProperty)
		if err != nil && !crosconfig.IsNotFound(err) {
			errors.Wrapf(err, "failed to get crosconfig %v property", crosConfigProperty)
		}
		hasProperty := err == nil && val == "true"
		if actual, ok := actualValue.(string); ok {
			if !hasProperty {
				if actual != "NA" {
					return errors.Errorf("failed to get correct value: got %v, want NA", actual)
				}
				return nil
			}
			expectedValueByteArr, err := ioutil.ReadFile(filePathAndName)
			if err != nil {
				return errors.Wrapf(err, "failed to read file %v", filePathAndName)
			}
			expected := strings.TrimRight(string(expectedValueByteArr), "\n")
			if actual != expected {
				return errors.Errorf("failed to get correct value: got %v, want %v", actual, expected)
			}
			return nil
		}
		return errors.Errorf("invalid parameter type in CheckFileContentIfFileShouldExist: got %T, want string", actualValue)
	}
}

// EqualToCrosConfigContent returns a function that uses crosconfig and its two
// arguments, |crosConfigProperty| and |crosConfigPath| to obtain a value that
// is compared with |actualValue|.
func EqualToCrosConfigContent(ctx context.Context, crosConfigPath, crosConfigProperty string) func(interface{}) error {
	return func(actualValue interface{}) error {
		expected, err := crosconfig.Get(ctx, crosConfigPath, crosConfigProperty)
		if err != nil && !crosconfig.IsNotFound(err) {
			errors.Wrapf(err, "failed to get crosconfig %v property", crosConfigProperty)
		}
		if actual, ok := actualValue.(string); ok {
			if actual != expected {
				return errors.Errorf("failed to get correct value: got %v, want %v", actual, expected)
			}
			return nil
		}
		return errors.Errorf("invalid parameter type in EqualToCrosConfigContent: got %T, want string", actualValue)
	}
}
