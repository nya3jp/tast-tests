// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package systemlogs calls autotestPrivate.writeSystemLogs and parses the results.
package systemlogs

import (
	"context"
	"io/ioutil"
	"os"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
)

// GetSystemLogs returns a string containing the complete contents of the
// system logs file exported by chrome.autotestPrivate.writeSystemLogs.
// The logs are written to a file in the /tmp directory which is removed
// after this returns.
func GetSystemLogs(ctx context.Context, tconn *chrome.TestConn) (string, error) {
	dest := "/tmp"
	var err error
	var filepath string
	if err = tconn.Call(ctx, &filepath, `tast.promisify(chrome.autotestPrivate.writeSystemLogs)`, dest); err != nil {
		return "", err
	}
	defer os.Remove(filepath)

	if !strings.HasSuffix(filepath, ".zip") {
		return "", errors.New("system_logs file is not zipped")
	}
	if err := testexec.CommandContext(ctx, "unzip", "-u", filepath, "-d", dest).Run(testexec.DumpLogOnError); err != nil {
		return "", errors.Wrap(err, "failed to unzip")
	}

	txtFilepath := filepath[0 : len(filepath)-4]
	var systemLogs []byte
	if systemLogs, err = ioutil.ReadFile(txtFilepath); err != nil {
		return "", err
	}
	defer os.Remove(txtFilepath)
	return string(systemLogs), nil
}

// GetMultilineSection exports the ststem logs from chrome by calling GetSystemLogs,
// then searches for the provided multiline section. If found, returns the
// contents of the section between the START and END markers as a string.
func GetMultilineSection(ctx context.Context, tconn *chrome.TestConn, section string) (string, error) {
	var err error
	var logs string
	if logs, err = GetSystemLogs(ctx, tconn); err != nil {
		return "", err
	}

	var index, start, end int
	key := section + "=<multiline>"
	if index = strings.Index(logs, key); index == -1 {
		return "", errors.Errorf("System logs missing section: %s", section)
	}
	logs = logs[index:]
	const startKey = "---------- START ----------"
	if start = strings.Index(logs, startKey); start == -1 {
		return "", errors.New("System logs missing start key")
	}
	start += len(startKey)
	const endKey = "---------- END ----------"
	if end = strings.Index(logs, endKey); end == -1 {
		return "", errors.New("System logs missing end key")
	}
	return logs[start:end], nil
}
