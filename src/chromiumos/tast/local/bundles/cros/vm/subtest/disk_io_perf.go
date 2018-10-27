// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package subtest

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func runOnce(ctx context.Context, s *testing.State, cont *vm.Container) error {
	cmd := cont.Command(ctx,
		"fio", "-filename=fio_test_data", "-size=1G", "-bs=1m", "-readwrite=write", "-ioengine=libaio", "-iodepth=1", "-direct=1", "-name=test", "-loops=15")
	out, err := cmd.Output()
	if err != nil {
		s.Log("out: ", string(out))
		cmd.DumpLog(ctx)
		return errors.Wrap(err, "failed to run fio")
	}

	return nil
}

// DiskIOPerf measure disk performance.
func DiskIOPerf(ctx context.Context, s *testing.State, cont *vm.Container) error {
	s.Log("Measuring disk IO performance")

	s.Log("Installing fio")
	cmd := cont.Command(ctx, "sh", "-c", "yes | sudo apt-get install fio")
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "failed to install fio")
	}

	const numTries = 100

	for i := 0; i < numTries; i++ {
		err := runOnce(ctx, s, cont)
		if err != nil {
			return err
		}
	}
	return nil
}
