// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"context"
	"path"
)

const (
	testBinaryDir = "/"
)

func getChromeBinaryPath(binaryToRun string) string {
	const testBinaryDir = "/usr/local/share/tast/chrome_binary/"
	return path.Join(testBinaryDir, binaryToRun)
}

func RunChromeTestBinary(ctx context.Context, binaryToRun string, extraParams string) error {
	binaryPath = getChromeBinaryPath(binaryToRun)
	return nil
}
