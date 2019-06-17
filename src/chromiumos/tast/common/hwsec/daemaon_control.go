// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
)

func StartCryptohome(ctx context.Context) {
	call(ctx, "start", "cryptohome")
}
func StopCryptohome(ctx context.Context) {
	call(ctx, "stop", "cryptohome")
}
func RestartCryptohome(ctx context.Context) {
	call(ctx, "restart", "cryptohome")
}
