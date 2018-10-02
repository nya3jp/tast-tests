// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package cpu provides utility function for cpu.
package cpu

import (
	"context"
	"time"
)

// WaitForIdle waits until cpu is idle, or timeout is elapsed.
// CPU is evaulated as idle if the cpu usage (percent) is less than utilization.
func WaitForIdle(ctx context.Context, timeout time.Duration, utilization float64) error {
	// TODO(crbug.com/890733): Implement wait_for_idle_cpu(),
	// https://chromium.googlesource.com/chromiumos/third_party/autotest/+/master/client/bin/utils.py#1624
	return nil
}
