// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memoryuser

import (
	"context"

	"chromiumos/tast/local/memory/mempressure"
)

// MemPressureTask implements MemoryTask to create memory pressure by opening Chrome tabs.
type MemPressureTask struct {
	Params *mempressure.RunParameters
}

// Run starts the platform.MemoryPressure test, creating memory pressure by opening Chrome tabs
func (mpt *MemPressureTask) Run(ctx context.Context, testEnv *TestEnv) error {
	return mempressure.Run(ctx, testEnv.outDir, testEnv.cr, testEnv.arc, mpt.Params)
}

// Close does nothing, the Run method of platform.MemoryPressure already closes the connections
func (mpt *MemPressureTask) Close(ctx context.Context, testEnv *TestEnv) {
}

// String returns a string describing the MemPressureTask.
func (mpt *MemPressureTask) String() string {
	return "MemoryPressureTask"
}

// NeedVM returns false to indicate that no VM is required for a MemPressureTask.
func (mpt *MemPressureTask) NeedVM() bool {
	return false
}
