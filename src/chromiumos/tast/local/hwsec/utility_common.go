// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"time"

	"chromiumos/tast/testing"
)

// utilityCommon implements the common function shared across all
// implementations of |Utility|.
type utilityCommon struct {
	ctx context.Context
}

func (utility utilityCommon) sleep(millis int) error {
	return testing.Sleep(utility.ctx, time.Duration(millis)*time.Millisecond)
}
