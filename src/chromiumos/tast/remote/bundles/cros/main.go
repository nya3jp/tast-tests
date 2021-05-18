// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package main implements the "cros" remote test bundle.
//
// This executable contains standard Chrome OS tests.
package main

import (
	"chromiumos/tast/remote/bundlemain"

	// These packages register their tests via init functions.
	_ "chromiumos/tast/common/global"
	_ "chromiumos/tast/remote/bundles/cros/arc"
	_ "chromiumos/tast/remote/bundles/cros/camera"
	_ "chromiumos/tast/remote/bundles/cros/crash"
	_ "chromiumos/tast/remote/bundles/cros/example"
	_ "chromiumos/tast/remote/bundles/cros/factory"
	_ "chromiumos/tast/remote/bundles/cros/filemanager"
	_ "chromiumos/tast/remote/bundles/cros/firmware"
	_ "chromiumos/tast/remote/bundles/cros/hardware"
	_ "chromiumos/tast/remote/bundles/cros/hwsec"
	_ "chromiumos/tast/remote/bundles/cros/kernel"
	_ "chromiumos/tast/remote/bundles/cros/meta"
	_ "chromiumos/tast/remote/bundles/cros/nearbyshare"
	_ "chromiumos/tast/remote/bundles/cros/network"
	_ "chromiumos/tast/remote/bundles/cros/network/allowlist"
	_ "chromiumos/tast/remote/bundles/cros/platform"
	_ "chromiumos/tast/remote/bundles/cros/policy"
	_ "chromiumos/tast/remote/bundles/cros/power"
	_ "chromiumos/tast/remote/bundles/cros/security"
	_ "chromiumos/tast/remote/bundles/cros/syzcorpus"
	_ "chromiumos/tast/remote/bundles/cros/syzkaller"
	_ "chromiumos/tast/remote/bundles/cros/typec"
	_ "chromiumos/tast/remote/bundles/cros/ui"
	_ "chromiumos/tast/remote/bundles/cros/usbc"
	_ "chromiumos/tast/remote/bundles/cros/wifi"
	_ "chromiumos/tast/remote/bundles/cros/wilco"
)

func main() {
	bundlemain.RunRemote()
}
