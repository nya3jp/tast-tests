// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"bufio"
	"os"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
)

// readLine reads a line from a file.
// Line feed character will be removed to ease converting the string
// into other types.
func readLine(filePath string, line int) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	var lineContent string
	for i := 0; i < line+1; i++ {
		lineContent, err = reader.ReadString('\n')
		if err != nil && i < line {
			// Reader.ReadString returns error iff line does not end in \n.
			// The last line may not end in \n, but all other lines are.
			return "", errors.Errorf("file does not have enough lines: expected >= %d, found %d", line+1, i+1)
		}
	}

	return strings.TrimSuffix(lineContent, "\n"), nil
}

// readFloat64 reads a line from a file and converts it into float64.
func readFloat64(filePath string) (float64, error) {
	str, err := readLine(filePath, 0)
	if err != nil {
		return 0., err
	}
	return strconv.ParseFloat(str, 64)
}

// readInt64 reads a line from a file and converts it into int64.
func readInt64(filePath string) (int64, error) {
	str, err := readLine(filePath, 0)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(str, 10, 64)
}
