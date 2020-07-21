// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package systemlogs calls autotestPrivate.writeSystemLogs and parses the results.
package systemlogs

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
)

// systemInformation corresponds to feedbackPrivate.SystemInformation entries.
type systemInformation struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// GetSystemLogs returns a string containing the complete contents of the
// system logs file exported by chrome.autotestPrivate.writeSystemLogs.
// The logs are written to a file in the /tmp directory which is removed
// after this returns.
func GetSystemLogs(ctx context.Context, tconn *chrome.TestConn, key string) (string, error) {
	var systemInfo []*systemInformation
	if err := tconn.Call(ctx, &systemInfo, `tast.promisify(chrome.feedbackPrivate.getSystemInformation)`); err != nil {
		return "", err
	}
	for _, info := range systemInfo {
		if info.Key == key {
			return info.Value, nil
		}
	}
	return "", errors.Errorf("key not found: %q", key)
}
