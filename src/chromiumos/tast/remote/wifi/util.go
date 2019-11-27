// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"context"

	"chromiumos/tast/dut"
)

func writeToDUT(ctx context.Context, dut *dut.DUT, path string, data []byte) error {
	// TODO: there must be some better way for this...
	tee := dut.Command("tee", path)
	pipe, err := tee.StdinPipe()
	if err != nil {
		return err
	}
	if err := tee.Start(ctx); err != nil {
		return nil
	}
	defer tee.Wait(ctx)
	if _, err := pipe.Write(data); err != nil {
		tee.Abort()
		return err
	}
	pipe.Close()
	return nil
}
