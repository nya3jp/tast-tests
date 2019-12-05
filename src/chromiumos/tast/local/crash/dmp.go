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

// checkNextDmpKeyForPID scans through a breakpad/crashpad-style .dmp file
// looking for a field that has a key of "pid" and a value of passed-in pid.
// .dmp file format is key:length:value. Note that there is no delimiter between
// a value and the next key, so we must parse the length even of keys we don't care about.
func checkNextDmpKeyForPID(file *bufio.Reader, pid string, bytesRead *int) (found, eof bool, err error) {
	const pidKey = "pid"
	key, err := file.ReadString(':')
	if err == io.EOF {
		return false, true, nil
	} else if err != nil {
		return false, false, errors.Wrapf(err, "could not read key from dmp file at %d", *bytesRead)
	}
	*bytesRead += len(key)
	key = strings.TrimSuffix(key, ":")
	lengthStr, err := file.ReadString(':')
	if err != nil {
		return false, false, errors.Wrapf(err, "could not read length from dmp file at %d", *bytesRead)
	}
	length, err := strconv.Atoi(strings.TrimSuffix(lengthStr, ":"))
	if err != nil {
		return false, false, errors.Wrapf(err, "could not parse length %q from dmp file at %d", lengthStr, *bytesRead)
	}
	*bytesRead += len(lengthStr) + 1
	if key == pidKey {
		buf := make([]byte, length)
		if _, err = io.ReadFull(file, buf); err != nil {
			return false, false, errors.Wrapf(err, "could not read pid value from dmp file at %d", *bytesRead)
		}
		*bytesRead += length
		if string(buf) == pid {
			// Success!
			return true, false, nil
		}
	} else {
		if _, err = file.Discard(length); err != nil {
			return false, false, errors.Wrapf(err, "could not skip expected %d bytes from dmp file starting at %d", length, *bytesRead)
		}
		*bytesRead += length
	}

	return false, false, nil
}

// IsBreakpadDmpFileForPID scans the given breakpad/crashpad format .dmp file
// to see if it is the minidump file for the pid process ID. It returns true
// if the file is the minidump for that process ID.
//
// This only works for the .dmp files created directly by breakpad or crashpad. It
// does not work for the .dmp files created by crash_reporter.
func IsBreakpadDmpFileForPID(fileName string, pid int) (bool, error) {
	pidString := strconv.Itoa(pid)
	osFile, err := os.Open(fileName)
	if err != nil {
		return false, errors.Wrap(err, "could not open dmp file")
	}
	defer osFile.Close()
	file := bufio.NewReader(osFile)
	found := false
	eof := false
	bytesRead := 0
	for !found && !eof {
		found, eof, err = checkNextDmpKeyForPID(file, pidString, &bytesRead)
		if err != nil {
			return false, errors.Wrap(err, "problems parsing dmp file")
		}
	}
	return found, nil
}
