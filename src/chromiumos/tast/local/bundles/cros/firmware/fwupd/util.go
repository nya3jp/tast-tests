// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fwupd

import (
	"context"
	"regexp"
	"strings"

	"chromiumos/tast/common/testexec"
)

// ReleaseURI runs the fwupdmgr utility and obtains the release
// URI of the test webcam device in the system.
func ReleaseURI(ctx context.Context) (string, error) {
	// b585990a-003e-5270-89d5-3705a17f9a43 is the GUID for a fake device.
	cmd := testexec.CommandContext(ctx, "fwupdmgr", "get-releases", "-v", "b585990a-003e-5270-89d5-3705a17f9a43", "--ignore-power")
	output, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return "", err
	}

	re := regexp.MustCompile(`.*Uri:.*`)
	return strings.Fields(string(re.Find(output)))[1], nil
}
