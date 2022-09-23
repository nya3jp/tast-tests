// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fingerprint

import (
	"context"

	fp "chromiumos/tast/common/fingerprint"
	"chromiumos/tast/dut"
)

const (
	rollbackFlashOffsetBloonchipper = "0x20000"
	rollbackFlashOffsetDartmonkey   = "0xe0000"
)

func rollbackFlashOffset(fpBoard fp.FPBoardName) string {
	if fpBoard == fp.FPBoardNameBloonchipper {
		return rollbackFlashOffsetBloonchipper
	}
	return rollbackFlashOffsetDartmonkey
}

// ReadFromRollbackFlash attempts to read bytes from the rollback section of the FPMCU's flash.
// The directory containing outputFile must already exist on the DUT.
func ReadFromRollbackFlash(ctx context.Context, d *dut.DUT, fpBoard fp.FPBoardName, outputFile string) error {
	offset := rollbackFlashOffset(fpBoard)
	return EctoolCommand(ctx, d, "flashread", offset, "0x1000", outputFile).Run()
}
