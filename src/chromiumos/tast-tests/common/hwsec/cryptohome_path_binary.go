// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
)

// cryptohomePathBinary is used to interact with the 'cryptohome-path' executable,
// which provides an interface to libbrillo for retrieving the user's home path.
// For more details of the arguments of the functions in this file,
// please check //src/platform2/cryptohome/cryptohome-path.cc.
// The arguments here are documented only when they are not directly
// mapped to the ones in so-mentioned cryptohome-path.cc.
type cryptohomePathBinary struct {
	runner CmdRunner
}

// newCryptohomePathBinary is a factory function that create a
// cryptohomePathBinary instance.
func newCryptohomePathBinary(r CmdRunner) *cryptohomePathBinary {
	return &cryptohomePathBinary{r}
}

func (c *cryptohomePathBinary) call(ctx context.Context, args ...string) ([]byte, error) {
	return c.runner.Run(ctx, "cryptohome-path", args...)
}

// userPath calls "cryptohome-path user <username>" to retrieve the user home for the user.
func (c *cryptohomePathBinary) userPath(ctx context.Context, username string) ([]byte, error) {
	return c.call(ctx, "user", username)
}

// systemPath calls "cryptohome-path system <username>" to retrieve the user root for the user.
func (c *cryptohomePathBinary) systemPath(ctx context.Context, username string) ([]byte, error) {
	return c.call(ctx, "system", username)
}
