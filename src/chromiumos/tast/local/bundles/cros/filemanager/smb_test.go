// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package filemanager

import (
	"testing"

	"chromiumos/tast/common/genparams"
	"chromiumos/tast/local/crostini"
)

func TestSmbParams(t *testing.T) {
	params := crostini.MakeTestParams(t)
	genparams.Ensure(t, "smb.go", params)
}
