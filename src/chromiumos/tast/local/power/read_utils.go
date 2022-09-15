// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"bufio"
	"os"
	"strconv"

	"chromiumos/tast/errors"
)

// readFirstLine reads the first line from a file.
// Line feed character will be removed to ease converting the string
// into other types.
func readFirstLine(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		return scanner.Text(), nil
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", errors.Errorf("found no content in %q", filePath)
}

// readFloat64 reads a line from a file and converts it into float64.
func readFloat64(filePath string) (float64, error) {
	str, err := readFirstLine(filePath)
	if err != nil {
		return 0., err
	}
	return strconv.ParseFloat(str, 64)
}

// readInt64 reads a line from a file and converts it into int64.
func readInt64(filePath string) (int64, error) {
	str, err := readFirstLine(filePath)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(str, 10, 64)
}
