// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package power interacts with power management on behalf of local tests.
package power

import (
	"context"
	"runtime"
	"time"
)

// FullyLoadCpus puts a load on all available CPUs on a device. It will stop
// after the first of:
// -The supplied context expires
// -The supplied timeout occurs
// -The returned cancel function is called
func FullyLoadCpus(ctx context.Context, timeout time.Duration) context.CancelFunc {
	numCpus := runtime.NumCPU()
	runtime.GOMAXPROCS(numCpus)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	for i := 0; i < numCpus; i++ {
		go func() {
			for {
				for i := 0; i < 2147483647; i++ {
				}
				select {
				case <-ctx.Done():
					return
				default:
					// Give up the thread to another goroutine
					runtime.Gosched()
				}
			}
		}()
	}

	return func() { cancel() }
}
