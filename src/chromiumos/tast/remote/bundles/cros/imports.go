// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	// These packages register their tests via init functions.
	_ "chromiumos/tast/remote/bundles/cros/apps"
	_ "chromiumos/tast/remote/bundles/cros/arc"
	_ "chromiumos/tast/remote/bundles/cros/audio"
	_ "chromiumos/tast/remote/bundles/cros/autoupdate"
	_ "chromiumos/tast/remote/bundles/cros/bluetooth"
	_ "chromiumos/tast/remote/bundles/cros/camera"
	_ "chromiumos/tast/remote/bundles/cros/cellular"
	_ "chromiumos/tast/remote/bundles/cros/crash"
	_ "chromiumos/tast/remote/bundles/cros/enterprise"
	_ "chromiumos/tast/remote/bundles/cros/example"
	_ "chromiumos/tast/remote/bundles/cros/factory"
	_ "chromiumos/tast/remote/bundles/cros/feedback"
	_ "chromiumos/tast/remote/bundles/cros/filemanager"
	_ "chromiumos/tast/remote/bundles/cros/firmware"
	_ "chromiumos/tast/remote/bundles/cros/firmware/utils"
	_ "chromiumos/tast/remote/bundles/cros/hardware"
	_ "chromiumos/tast/remote/bundles/cros/hps"
	_ "chromiumos/tast/remote/bundles/cros/hwsec"
	_ "chromiumos/tast/remote/bundles/cros/hypervisor"
	_ "chromiumos/tast/remote/bundles/cros/inputs"
	_ "chromiumos/tast/remote/bundles/cros/intel"
	_ "chromiumos/tast/remote/bundles/cros/kernel"
	_ "chromiumos/tast/remote/bundles/cros/lacros"
	_ "chromiumos/tast/remote/bundles/cros/meta"
	_ "chromiumos/tast/remote/bundles/cros/nearbyshare"
	_ "chromiumos/tast/remote/bundles/cros/network"
	_ "chromiumos/tast/remote/bundles/cros/network/allowlist"
	_ "chromiumos/tast/remote/bundles/cros/omaha"
	_ "chromiumos/tast/remote/bundles/cros/osinstall"
	_ "chromiumos/tast/remote/bundles/cros/platform"
	_ "chromiumos/tast/remote/bundles/cros/policy"
	_ "chromiumos/tast/remote/bundles/cros/power"
	_ "chromiumos/tast/remote/bundles/cros/sdcard"
	_ "chromiumos/tast/remote/bundles/cros/security"
	_ "chromiumos/tast/remote/bundles/cros/shimlessrma"
	_ "chromiumos/tast/remote/bundles/cros/spera"
	_ "chromiumos/tast/remote/bundles/cros/syzcorpus"
	_ "chromiumos/tast/remote/bundles/cros/syzkaller"
	_ "chromiumos/tast/remote/bundles/cros/typec"
	_ "chromiumos/tast/remote/bundles/cros/ui"
	_ "chromiumos/tast/remote/bundles/cros/usbc"
	_ "chromiumos/tast/remote/bundles/cros/wifi"
	_ "chromiumos/tast/remote/bundles/cros/wilco"
	_ "chromiumos/tast/remote/bundles/cros/wwcb"

	_ "chromiumos/tast/remote/bundles/cros/factory/fixture"
	_ "chromiumos/tast/remote/meta" // import fixture for meta tests
	_ "chromiumos/tast/remote/tape"
)
