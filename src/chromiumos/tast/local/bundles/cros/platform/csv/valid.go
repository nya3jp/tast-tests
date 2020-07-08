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

// validator represents a function validator that takes in as input an element
// that is part of the csv data. If the validation fails, an error is returned.
// If the validation succeeds, nil is returned.
type validator func(string) error

// ValidateCSV is responsible for validating headers and all values of the CSV
// output. An error is returned if the headers are invalid or if any of the CSV
// values are incorrect as determined by the validators provided to verify a
// particular CSV value.
func ValidateCSV(csv [][]string,
	rows int,
	headers []string,
	columnFuncs ...validator) error {
	// Validate csv dimensions.
	if len(csv) != rows {
		return errors.Errorf("incorrect number of rows in csv: got %v, want %v", len(csv), rows)
	}
	if len(csv) > 0 && len(headers) != len(csv[0]) {
		return errors.Errorf("header size does not match number of number of csv columns: got %v, want %v", len(headers), len(csv[0]))
	}
	if len(csv) > 0 && len(columnFuncs) != len(csv[0]) {
		return errors.Errorf("number of Column validators does not match number of csv columns: got %v, want %v", len(columnFuncs), len(csv[0]))
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

// Rows simply returns the number of rows (including header) that the csv must
// contain.
func Rows(expectedRows int) int {
	return expectedRows
}

// Headers returns a slice of the headers.
func Headers(headers ...string) []string {
	return headers
}

// Column returns a validator that will use a list of validators to validate
// all values in specific CSV column.
func Column(validators ...validator) validator {
	return func(csvValue string) error {
		for _, validator := range validators {
			if err := validator(csvValue); err != nil {
				return err
			}
		}
		return nil
	}
}

// UInt64 returns a validator that checks whether |actual| can be parsed
// into a uint64.
func UInt64() validator {
	return func(actual string) error {
		if _, err := strconv.ParseUint(actual, 10, 64); err != nil {
			if err != nil {
				return errors.Wrapf(err, "failed to convert %v to uint64", actual)
			}
		}
		return nil
	}
}

// MatchRegexOrNA returns a function that checks whether |actual| matches the
// regex pattern specified by |regex|. If |actual| is "NA", do not proceed
// with the pattern matching.
func MatchRegexOrNA(regex *regexp.Regexp) validator {
	return func(actual string) error {
		// If the value does not exist, do not check the format.
		if actual == "NA" {
			return nil
		}
		if !regex.MatchString(actual) {
			return errors.Errorf("failed to follow correct pattern: got %v, want %v", regex, actual)
		}
		return nil
	}
}

// EqualToFileContentOrNA returns a function that checks whether
// |path| exists. If it does, it compares the value at that location
// wih |actual|. If it does not, it ensures that |actual| equals
// "NA".
func EqualToFileContentOrNA(path string) validator {
	return func(actual string) error {
		expectedBytes, err := ioutil.ReadFile(path)
		if os.IsNotExist(err) {
			if actual != "NA" {
				return errors.Errorf("failed to get correct value: got %v, want NA", actual)
			}
			return nil
		} else if err != nil {
			return errors.Wrapf(err, "failed to read from file %v", path)
		}
		expected := strings.TrimRight(string(expectedBytes), "\n")
		if actual != expected {
			return errors.Errorf("value does not match content of %v: got %v, want %v", path, actual, expected)
		}
		return nil
	}
}

// EqualToFileIfCrosConfigPropOrNA returns a function that checks
// whether |filePath| should exist by using crosconfig and its two
// arguments, |prop| and |path|. If the file should
// exist, it attempts to read the value from the file. If it cannot, an error
// is reported. If it can, it compares the read value with |actual|.
func EqualToFileIfCrosConfigPropOrNA(ctx context.Context, path, prop, filePath string) validator {
	return func(actual string) error {
		val, err := crosconfig.Get(ctx, path, prop)
		if err != nil && !crosconfig.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get crosconfig %v property", prop)
		}
		// Property does not exist
		if crosconfig.IsNotFound(err) || val != "true" {
			if actual != "NA" {
				return errors.Errorf("failed to get correct value: got %v, want NA", actual)
			}
			return nil
		}
		expectedBytes, err := ioutil.ReadFile(filePath)
		if err != nil {
			return errors.Wrapf(err, "failed to read file %v", filePath)
		}
		expected := strings.TrimRight(string(expectedBytes), "\n")
		if actual != expected {
			return errors.Errorf("failed to get correct value: got %v, want %v", actual, expected)
		}
		return nil
	}
}

// EqualToCrosConfigProp returns a function that uses crosconfig and
// its two arguments, |prop| and |path| to obtain a
// value that is compared with |actual|.
func EqualToCrosConfigProp(ctx context.Context, path, prop string) validator {
	return func(actual string) error {
		expected, err := crosconfig.Get(ctx, path, prop)
		if err != nil && !crosconfig.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get crosconfig %v property", prop)
		}
		if actual != expected {
			return errors.Errorf("failed to get correct value: got %v, want %v", actual, expected)
		}
		return nil
	}
}
