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
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
)

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
	// Track # of bytes read. Since these .dmp files contain large blocks of very
	// repetitive binary data, error messages about parsing failures are
	// near-useless unless we print the offset where the parser failed.
	bytesRead := 0
	for {
		// A breakpad .dmp file is a series of key:length:value triplets.  Note that
		// there is no delimiter between a value and the next key, so we must parse
		// the length even of keys we don't care about.
		key, err := file.ReadString(':')
		if err == io.EOF && len(key) == 0 {
			// Just after the end of a value, and thus at the beginning of a key, is
			// where we expect the end of the file to be.
			return false, nil
		} else if err == io.EOF {
			// EOF partway through a key is not expected.
			return false, errors.Errorf("unexpected EOF while reading key at %d", bytesRead)
		} else if err != nil {
			return false, errors.Wrapf(err, "could not read key from dmp file at %d", bytesRead)
		}
		bytesRead += len(key)
		key = strings.TrimSuffix(key, ":")
		lengthStr, err := file.ReadString(':')
		if err != nil {
			return false, errors.Wrapf(err, "could not read length from dmp file at %d", bytesRead)
		}
		length, err := strconv.Atoi(strings.TrimSuffix(lengthStr, ":"))
		if err != nil {
			return false, errors.Wrapf(err, "could not parse length %q from dmp file at %d", lengthStr, bytesRead)
		}
		bytesRead += len(lengthStr) + 1
		if key != pidKey {
			// Some values are 100s of KB; Discard instead of wasting resources reading
			// data we don't actually care about.
			if _, err = file.Discard(length); err != nil {
				return false, errors.Wrapf(err, "could not skip expected %d bytes from dmp file starting at %d", length, bytesRead)
			}
			bytesRead += length
			continue
		}
		buf := make([]byte, length)
		if _, err = io.ReadFull(file, buf); err != nil {
			return false, errors.Wrapf(err, "could not read pid value from dmp file at %d", bytesRead)
		}
		bytesRead += length
		if string(buf) == pidString {
			// Success!
			return true, nil
		}
	}
}

// CheckMinidumpStackwalk acquires stack dump log from minidump and verifies it.
func CheckMinidumpStackwalk(ctx context.Context, minidumpPath, crasherPath, basename string, fromCrashReporter bool) error {
	symbolDir := filepath.Join(filepath.Dir(crasherPath), "symbols")
	command := []string{"minidump_stackwalk", minidumpPath, symbolDir}
	cmd := testexec.CommandContext(ctx, command[0], command[1:]...)
	out, err := cmd.CombinedOutput(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrapf(err, "failed to get minidump output %v", cmd)
	}
	if err := verifyStack(ctx, out, basename, fromCrashReporter); err != nil {
		return errors.Wrap(err, "minidump stackwalk verification failed")
	}
	return nil
}
