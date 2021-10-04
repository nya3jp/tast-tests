// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package testutil

import (
	"os"
)

// InChroot returns if the current environment is CrOS chroot.
func InChroot() bool {
	_, err := os.Stat("/etc/cros_chroot_version")
	return err == nil
}
