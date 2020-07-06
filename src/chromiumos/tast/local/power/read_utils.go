// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"io/ioutil"
	"strconv"
	"strings"
)

// readLine reads a file with single line content.
// Line feed character will be removed to ease converting the string
// into other types.
func readLine(filePath string) (string, error) {
	strBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(strBytes), "\n"), nil
}

// readFloat64 reads a line from a file and converts it into float64.
func readFloat64(filePath string) (float64, error) {
	str, err := readLine(filePath)
	if err != nil {
		return 0., err
	}
	return strconv.ParseFloat(str, 64)
}

// readInt64 reads a line from a file and converts it into int64.
func readInt64(filePath string) (int64, error) {
	str, err := readLine(filePath)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(str, 10, 64)
}
