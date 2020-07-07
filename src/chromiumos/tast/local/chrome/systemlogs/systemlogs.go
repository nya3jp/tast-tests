// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package systemlogs calls autotestPrivate.writeSystemLogs and parses the results.
package systemlogs

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
)

// GetSystemLogs returns a string containing the complete contents of the
// system logs file exported by chrome.autotestPrivate.writeSystemLogs.
// The logs are written to a file in the /tmp directory which is removed
// after this returns.
func GetSystemLogs(ctx context.Context, tconn *chrome.TestConn) (string, error) {
	// Use /tmp instead of ioutil.TempDir since chrome does not have permission
	// to write to the tast temp dir.
	const destPath = "/tmp"
	var zipFilepath string
	if err := tconn.Call(ctx, &zipFilepath, `tast.promisify(chrome.autotestPrivate.writeSystemLogs)`, destPath); err != nil {
		return "", errors.Wrap(err, "Failed to write system logs to: "+destPath)
	}
	defer os.Remove(zipFilepath)

	const zipExt = ".zip"
	if filepath.Ext(zipFilepath) != zipExt {
		return "", errors.New("system_logs file is not zipped")
	}
	if err := testexec.CommandContext(ctx, "unzip", "-u", zipFilepath, "-d", destPath).Run(testexec.DumpLogOnError); err != nil {
		return "", errors.Wrap(err, "failed to unzip")
	}

	txtFilepath := zipFilepath[0 : len(zipFilepath)-len(zipExt)]
	defer os.Remove(txtFilepath)

	systemLogs, err := ioutil.ReadFile(txtFilepath)
	if err != nil {
		return "", err
	}
	return string(systemLogs), nil
}
