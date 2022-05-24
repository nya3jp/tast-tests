// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	// These packages register their tests via init functions.
	_ "chromiumos/tast-tests/remote/bundles/cros/apps"
	_ "chromiumos/tast-tests/remote/bundles/cros/arc"
	_ "chromiumos/tast-tests/remote/bundles/cros/audio"
	_ "chromiumos/tast-tests/remote/bundles/cros/autoupdate"
	_ "chromiumos/tast-tests/remote/bundles/cros/camera"
	_ "chromiumos/tast-tests/remote/bundles/cros/cellular"
	_ "chromiumos/tast-tests/remote/bundles/cros/crash"
	_ "chromiumos/tast-tests/remote/bundles/cros/enterprise"
	_ "chromiumos/tast-tests/remote/bundles/cros/example"
	_ "chromiumos/tast-tests/remote/bundles/cros/factory"
	_ "chromiumos/tast-tests/remote/bundles/cros/filemanager"
	_ "chromiumos/tast-tests/remote/bundles/cros/firmware"
	_ "chromiumos/tast-tests/remote/bundles/cros/hardware"
	_ "chromiumos/tast-tests/remote/bundles/cros/hps"
	_ "chromiumos/tast-tests/remote/bundles/cros/hwsec"
	_ "chromiumos/tast-tests/remote/bundles/cros/inputs"
	_ "chromiumos/tast-tests/remote/bundles/cros/kernel"
	_ "chromiumos/tast-tests/remote/bundles/cros/lacros"
	_ "chromiumos/tast-tests/remote/bundles/cros/meta"
	_ "chromiumos/tast-tests/remote/bundles/cros/nearbyshare"
	_ "chromiumos/tast-tests/remote/bundles/cros/network"
	_ "chromiumos/tast-tests/remote/bundles/cros/network/allowlist"
	_ "chromiumos/tast-tests/remote/bundles/cros/omaha"
	_ "chromiumos/tast-tests/remote/bundles/cros/osinstall"
	_ "chromiumos/tast-tests/remote/bundles/cros/platform"
	_ "chromiumos/tast-tests/remote/bundles/cros/policy"
	_ "chromiumos/tast-tests/remote/bundles/cros/power"
	_ "chromiumos/tast-tests/remote/bundles/cros/sdcard"
	_ "chromiumos/tast-tests/remote/bundles/cros/security"
	_ "chromiumos/tast-tests/remote/bundles/cros/shimlessrma"
	_ "chromiumos/tast-tests/remote/bundles/cros/syzcorpus"
	_ "chromiumos/tast-tests/remote/bundles/cros/syzkaller"
	_ "chromiumos/tast-tests/remote/bundles/cros/typec"
	_ "chromiumos/tast-tests/remote/bundles/cros/ui"
	_ "chromiumos/tast-tests/remote/bundles/cros/usbc"
	_ "chromiumos/tast-tests/remote/bundles/cros/wifi"
	_ "chromiumos/tast-tests/remote/bundles/cros/wilco"

	_ "chromiumos/tast-tests/remote/bundles/cros/factory/fixture"
	_ "chromiumos/tast-tests/remote/meta" // import fixture for meta tests
)
