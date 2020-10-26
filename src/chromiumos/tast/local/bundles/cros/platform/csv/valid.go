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

type column struct {
	field      string
	columnFunc validator
}

// ValidateCSV is responsible for validating headers and all values of the CSV
// output. An error is returned if the headers are invalid or if any of the CSV
// values are incorrect as determined by the validators provided to verify a
// particular CSV value.
func ValidateCSV(csv [][]string, rows int, columns ...column) error {
	// Validate csv dimensions.
	if len(csv) != rows {
		return errors.Errorf("incorrect number of rows in csv: got %v, want %v", len(csv), rows)
	}
	if len(csv) == 0 {
		return nil
	}
	if len(columns) != len(csv[0]) {
		return errors.Errorf("number of Column validators does not match number of csv columns: got %v, want %v", len(columns), len(csv[0]))
	}
	// Validate headers.
	for i, col := range columns {
		if col.field != csv[0][i] {
			return errors.Errorf("incorrect header name: got %v, want %v", csv[0][i], col.field)
		}
	}

	// Validate all columns of the CSV (starting from first row) in |csv|.
	for i := 1; i < len(csv); i++ {
		for j, col := range columns {
			if err := col.columnFunc(csv[i][j]); err != nil {
				return errors.Wrapf(err, "failed validation on %v (Column %v) with value %v", col.field, j, csv[i][j])
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

// Column returns the field name of the column and a validator that will use a
// list of validators to validate all values in this CSV column. The return
// value is of the column type.
func Column(field string, validators ...validator) column {
	columnFunc := func(csvValue string) error {
		for _, validator := range validators {
			if err := validator(csvValue); err != nil {
				return err
			}
		}
		return nil
	}
	return column{field, columnFunc}
}

// ColumnWithDefault returns the field name of the column and a validator that
// will use a list of validators to validate all values in this CSV column. If
// the field value is equal to |defaultValue|, the validators are not run.
func ColumnWithDefault(field, defaultValue string, validators ...validator) column {
	columnFunc := func(csvValue string) error {
		if csvValue == defaultValue {
			return nil
		}

		for _, validator := range validators {
			if err := validator(csvValue); err != nil {
				return err
			}
		}
		return nil
	}
	return column{field, columnFunc}
}

// UInt64 returns a validator that checks whether |actual| can be parsed into a
// uint64.
func UInt64() validator {
	return func(actual string) error {
		if _, err := strconv.ParseUint(actual, 10, 64); err != nil {
			return errors.Wrapf(err, "failed to convert %v to uint64", actual)
		}
		return nil
	}
}

// MatchValue returns a validator that checks whether |value| is equal to
// |actual|.
func MatchValue(value string) validator {
	return func(actual string) error {
		if value != actual {
			return errors.Errorf("values do not match; got %v, want %v", value, actual)
		}
		return nil
	}
}

// MatchRegex returns a function that checks whether |actual| matches the
// regex pattern specified by |regex|.
func MatchRegex(regex *regexp.Regexp) validator {
	return func(actual string) error {
		if !regex.MatchString(actual) {
			return errors.Errorf("failed to follow correct pattern: got %v, want %v", regex, actual)
		}
		return nil
	}
}

// EqualToFileContent returns a function that checks whether |path| exists. If
// it does, it compares the value at that location wih |actual|. If it does not,
// an error is returned.
func EqualToFileContent(path string) validator {
	return func(actual string) error {
		expectedBytes, err := ioutil.ReadFile(path)
		if os.IsNotExist(err) {
			return errors.Errorf("file does not exist: %v", path)
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

// EqualToFileIfCrosConfigProp returns a function that checks whether
// |filePath| should exist by using crosconfig and its two arguments, |prop| and
// |path|. If the crosconfig property does not exist, an error is returned. If
// the file exists, it attempts to read the value from the file. If it
// cannot, an error is reported. If it can, it compares the read value with
// |actual|.
func EqualToFileIfCrosConfigProp(ctx context.Context, path, prop, filePath string) validator {
	return func(actual string) error {
		val, err := crosconfig.Get(ctx, path, prop)
		if err != nil && !crosconfig.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get crosconfig %v property", prop)
		}
		// Property does not exist
		if crosconfig.IsNotFound(err) || val != "true" {
			return errors.Errorf("crosconfig property does not exist: %v", prop)
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

// EqualToCrosConfigProp returns a function that uses crosconfig and its two
// arguments, |prop| and |path| to obtain a value that is compared with
// |actual|.
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
