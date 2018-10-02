// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cpuutil provides utility function for cpu.
package cpuutil

import (
	"context"
	"time"
)

func WaitForIdleCpu(ctx context.Context, timeout time.Duration, utilization float64) error {
	// TODO(crbug.com/890733): Implement wait_for_idle_cpu(),
	// https://chromium.googlesource.com/chromiumos/third_party/autotest/+/master/client/bin/utils.py#1624
	return nil
}
