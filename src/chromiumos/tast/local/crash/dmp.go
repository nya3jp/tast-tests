// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

// This file contains utilities that parse the .dmp files directly created by
// breakpad and crashpad. Not to be confused with the .dmp files created by
// crash_reporter. The function names are deliberately ugly to keep people from
// mistaking these for utilites for parsing the more normal crash_reporter files.

import (
	"bufio"
	"io"
	"os"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
)

// readNextDmpKeyValue reads through a breakpad/crashpad-style .dmp file and
// extracts the next key/value pair. .dmp file format is key:length:value. Note
// that there is no delimiter between a value and the next key, so we must parse
// all the key/value pairs even even if we don't care about them.
//
// On success, bytesRead is incremented by the number of bytes read while
// parsing this key/value pair. It should start at zero for each file and the
// same pointer should be passed to each call for the file. This improves the
// error messages, which can otherwise be uninterpretable.
//
// Return an error of io.EOF if there were no more key/value in the file.
// Returns non-io.EOF errors if there were other problems, including EOF if
// the middle of a key/value pair.
func readNextDmpKeyValue(file *bufio.Reader, bytesRead *int) (key, value string, err error) {
	key, err = file.ReadString(':')
	if err == io.EOF && len(key) == 0 {
		// Just after the end of a value, and thus at the beginning of a key, is
		// where we expect the end of the file to be.
		return "", "", io.EOF
	} else if err == io.EOF {
		// EOF partway through a key is not expected.
		return "", "", errors.Errorf("unexpected EOF while reading key at %d", *bytesRead)
	} else if err != nil {
		return "", "", errors.Wrapf(err, "could not read key from dmp file at %d", *bytesRead)
	}
	*bytesRead += len(key)
	key = strings.TrimSuffix(key, ":")
	lengthStr, err := file.ReadString(':')
	if err != nil {
		return "", "", errors.Wrapf(err, "could not read length from dmp file at %d", *bytesRead)
	}
	length, err := strconv.Atoi(strings.TrimSuffix(lengthStr, ":"))
	if err != nil {
		return "", "", errors.Wrapf(err, "could not parse length %q from dmp file at %d", lengthStr, *bytesRead)
	}
	*bytesRead += len(lengthStr) + 1
	valueBuf := make([]byte, length)
	if _, err = io.ReadFull(file, valueBuf); err != nil {
		return "", "", errors.Wrapf(err, "could not read pid value from dmp file at %d", *bytesRead)
	}
	*bytesRead += length

	return key, string(valueBuf), nil
}

// IsBreakpadDmpFileForPID scans the given breakpad/crashpad format .dmp file
// to see if it is the minidump file for the pid process ID. It returns true
// if the file is the minidump for that process ID.
//
// This only works for the .dmp files created directly by breakpad or crashpad. It
// does not work for the .dmp files created by crash_reporter.
func IsBreakpadDmpFileForPID(fileName string, pid int) (bool, error) {
	const pidKey = "pid"
	pidString := strconv.Itoa(pid)
	osFile, err := os.Open(fileName)
	if err != nil {
		return false, errors.Wrap(err, "could not open dmp file")
	}
	defer osFile.Close()
	file := bufio.NewReader(osFile)
	bytesRead := 0
	for {
		key, value, err := readNextDmpKeyValue(file, &bytesRead)
		if err == io.EOF {
			// Expected end of file. Do not return an error.
			return false, nil
		} else if err != nil {
			return false, errors.Wrap(err, "problems parsing dmp file")
		} else if key == "pid" && value == pidString {
			return true, nil
		}
	}
}
