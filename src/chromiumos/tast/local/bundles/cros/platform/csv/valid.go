// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package csv provides methods for easily validating the rows and columns of a
// CSV file.
package csv

import (
	"context"
	"io/ioutil"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/crosconfig"
)

// Validator represents a function validator that takes in as input an element
// that is part of the csv data. If the validation fails, an error is returned.
// If the validation succeeds, nil is returned.
type Validator func(string) error

// ValidateCSV is responsible for validating headers and all values of the CSV
// output. An error is returned if the headers are invalid or if any of the CSV
// values are incorrect as determined by the validators provided to verify a
// particular CSV value.
func ValidateCSV(csv [][]string,
	dimensionFunc func([][]string, []string, []Validator) error,
	headers []string,
	columnFuncs ...Validator) error {
	// Validate csv dimensions.
	if err := dimensionFunc(csv, headers, columnFuncs); err != nil {
		return errors.Wrap(err, "incorrect csv dimensions")
	}
	if len(csv) == 0 {
		return nil
	}
	// Validate headers
	if ok := reflect.DeepEqual(csv[0], headers); !ok {
		return errors.Errorf("incorrect headers: got %v, want %v", csv[0], headers)
	}
	// For every row (starting from first row) in |csv|, validate its columns.
	for i := 1; i < len(csv); i++ {
		for j, columnFunc := range columnFuncs {
			if err := columnFunc(csv[i][j]); err != nil {
				return errors.Wrapf(err, "failed validation on %v (Column %v) with value %v", headers[j], j, csv[i][j])
			}
		}
	}
	return nil
}

// Dimensions takes the expected number of rows the CSV should contain
// (including header) and returns a function that does varios dimension checks.
// The function will be invoked in ValidateCSV() with a CSV of type [][]string.
func Dimensions(expectedRows int) func([][]string, []string, []Validator) error {
	return func(csv [][]string, headers []string, columnFuncs []Validator) error {
		if len(csv) != expectedRows {
			return errors.Errorf("incorrect number of rows in csv: got %v, want %v", len(csv), expectedRows)
		}
		if len(csv) > 0 && len(headers) != len(csv[0]) {
			return errors.Errorf("header size does not match number of number of csv columns: got %v, want %v", len(headers), len(csv[0]))
		}
		if len(csv) > 0 && len(columnFuncs) != len(csv[0]) {
			return errors.Errorf("number of Column validators does not match number of csv columns: got %v, want %v", len(columnFuncs), len(csv[0]))
		}
		return nil
	}
}

// Headers returns a slice of the headers.
func Headers(headers ...string) []string {
	return headers
}

// Column takes in a list of validators that will verify different properties
// of |csvValue|. Column returns a function that will be invoked in
// ValidateCSV().
func Column(validators ...Validator) Validator {
	return func(csvValue string) error {
		for _, validator := range validators {
			if err := validator(csvValue); err != nil {
				return err
			}
		}
		return nil
	}
}

// UInt64 returns a function that checks whether |actualValue| can be parsed
// into a uint64.
func UInt64() Validator {
	return func(actualValue string) error {
		// Confirm |actualValue| can be converted to uint64
		_, err := strconv.ParseUint(actualValue, 10, 64)
		if err != nil {
			return errors.Wrapf(err, "failed to convert %v to uint64", actualValue)
		}
		return nil
	}
}

// TelemMatchRegex returns a function that checks whether |actualValue| matches the
// regex pattern specified by |regex|. If |actualValue| is "NA", do not proceed
// with the pattern matching.
func TelemMatchRegex(regex *regexp.Regexp) Validator {
	return func(actualValue string) error {
		// If the value does not exist, do not check the format.
		if actualValue == "NA" {
			return nil
		}
		if matched := regex.MatchString(actualValue); !matched {
			return errors.Errorf("failed to follow correct pattern: got %v, want %v", regex, actualValue)
		}
		return nil
	}
}

// TelemEqualToFileContent returns a function that checks whether |filePathAndName|
// and exists. If it does, it compares the value at that location wih
// |actualValue|. If it does not, it ensures that |actualValue| equals "NA".
func TelemEqualToFileContent(filePathAndName string) Validator {
	return func(actualValue string) error {
		expectedValueByteArr, err := ioutil.ReadFile(filePathAndName)
		if os.IsNotExist(err) {
			if actualValue != "NA" {
				return errors.Errorf("failed to get correct value: got %v, want NA", actualValue)
			}
			return nil
		} else if err != nil {
			return errors.Wrapf(err, "failed to read from file %v", filePathAndName)
		}
		expectedValue := strings.TrimRight(string(expectedValueByteArr), "\n")
		if actualValue != expectedValue {
			return errors.Errorf("value does not match content of %v: got %v, want %v", filePathAndName, actualValue, expectedValue)
		}
		return nil
	}
}

// TelemCheckFileContentIfFileShouldExist returns a function that checks
// whether |filePathAndName| should exist by using crosconfig and its two
// arguments, |crosConfigProperty| and |crosConfigPath|. If the file should
// exist, it attempts to read the value from the file. If it cannot, an error
// is reported. If it can, it compares the read value with |actualValue|.
func TelemCheckFileContentIfFileShouldExist(ctx context.Context, crosConfigPath,
	crosConfigProperty, filePathAndName string) Validator {
	return func(actualValue string) error {
		val, err := crosconfig.Get(ctx, crosConfigPath, crosConfigProperty)
		if err != nil && !crosconfig.IsNotFound(err) {
			errors.Wrapf(err, "failed to get crosconfig %v property", crosConfigProperty)
		}
		hasProperty := err == nil && val == "true"
		if !hasProperty {
			if actualValue != "NA" {
				return errors.Errorf("failed to get correct value: got %v, want NA", actualValue)
			}
			return nil
		}
		expectedValueByteArr, err := ioutil.ReadFile(filePathAndName)
		if err != nil {
			return errors.Wrapf(err, "failed to read file %v", filePathAndName)
		}
		expectedValue := strings.TrimRight(string(expectedValueByteArr), "\n")
		if actualValue != expectedValue {
			return errors.Errorf("failed to get correct value: got %v, want %v", actualValue, expectedValue)
		}
		return nil
	}
}

// TelemEqualToCrosConfigContent returns a function that uses crosconfig and
// its two arguments, |crosConfigProperty| and |crosConfigPath| to obtain a
// value that is compared with |actualValue|.
func TelemEqualToCrosConfigContent(ctx context.Context, crosConfigPath,
	crosConfigProperty string) Validator {
	return func(actualValue string) error {
		expectedValue, err := crosconfig.Get(ctx, crosConfigPath, crosConfigProperty)
		if err != nil && !crosconfig.IsNotFound(err) {
			errors.Wrapf(err, "failed to get crosconfig %v property", crosConfigProperty)
		}
		if actualValue != expectedValue {
			return errors.Errorf("failed to get correct value: got %v, want %v", actualValue, expectedValue)
		}
		return nil
	}
}
