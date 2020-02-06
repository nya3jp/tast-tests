// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dutfs

import (
	"os"
	gotesting "testing"
)

func expectOsFileInfo(x os.FileInfo) {
}

func TestFileInfoImplementsOsFileInfo(t *gotesting.T) {
	// fileInfo should implement os.FileInfo interface.
	var f fileInfo
	// Assert that dutfs.fileInfo implements os.FileInfo
	expectOsFileInfo(f)
}
