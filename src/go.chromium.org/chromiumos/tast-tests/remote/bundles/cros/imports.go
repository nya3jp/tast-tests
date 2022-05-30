// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	// These packages register their tests via init functions.
	_ "go.chromium.org/chromiumos/tast-tests/remote/bundles/cros/apps"
	_ "go.chromium.org/chromiumos/tast-tests/remote/bundles/cros/arc"
	_ "go.chromium.org/chromiumos/tast-tests/remote/bundles/cros/audio"
	_ "go.chromium.org/chromiumos/tast-tests/remote/bundles/cros/autoupdate"
	_ "go.chromium.org/chromiumos/tast-tests/remote/bundles/cros/camera"
	_ "go.chromium.org/chromiumos/tast-tests/remote/bundles/cros/cellular"
	_ "go.chromium.org/chromiumos/tast-tests/remote/bundles/cros/crash"
	_ "go.chromium.org/chromiumos/tast-tests/remote/bundles/cros/enterprise"
	_ "go.chromium.org/chromiumos/tast-tests/remote/bundles/cros/example"
	_ "go.chromium.org/chromiumos/tast-tests/remote/bundles/cros/factory"
	_ "go.chromium.org/chromiumos/tast-tests/remote/bundles/cros/filemanager"
	_ "go.chromium.org/chromiumos/tast-tests/remote/bundles/cros/firmware"
	_ "go.chromium.org/chromiumos/tast-tests/remote/bundles/cros/hardware"
	_ "go.chromium.org/chromiumos/tast-tests/remote/bundles/cros/hps"
	_ "go.chromium.org/chromiumos/tast-tests/remote/bundles/cros/hwsec"
	_ "go.chromium.org/chromiumos/tast-tests/remote/bundles/cros/inputs"
	_ "go.chromium.org/chromiumos/tast-tests/remote/bundles/cros/kernel"
	_ "go.chromium.org/chromiumos/tast-tests/remote/bundles/cros/lacros"
	_ "go.chromium.org/chromiumos/tast-tests/remote/bundles/cros/meta"
	_ "go.chromium.org/chromiumos/tast-tests/remote/bundles/cros/nearbyshare"
	_ "go.chromium.org/chromiumos/tast-tests/remote/bundles/cros/network"
	_ "go.chromium.org/chromiumos/tast-tests/remote/bundles/cros/network/allowlist"
	_ "go.chromium.org/chromiumos/tast-tests/remote/bundles/cros/omaha"
	_ "go.chromium.org/chromiumos/tast-tests/remote/bundles/cros/osinstall"
	_ "go.chromium.org/chromiumos/tast-tests/remote/bundles/cros/platform"
	_ "go.chromium.org/chromiumos/tast-tests/remote/bundles/cros/policy"
	_ "go.chromium.org/chromiumos/tast-tests/remote/bundles/cros/power"
	_ "go.chromium.org/chromiumos/tast-tests/remote/bundles/cros/sdcard"
	_ "go.chromium.org/chromiumos/tast-tests/remote/bundles/cros/security"
	_ "go.chromium.org/chromiumos/tast-tests/remote/bundles/cros/shimlessrma"
	_ "go.chromium.org/chromiumos/tast-tests/remote/bundles/cros/syzcorpus"
	_ "go.chromium.org/chromiumos/tast-tests/remote/bundles/cros/syzkaller"
	_ "go.chromium.org/chromiumos/tast-tests/remote/bundles/cros/typec"
	_ "go.chromium.org/chromiumos/tast-tests/remote/bundles/cros/ui"
	_ "go.chromium.org/chromiumos/tast-tests/remote/bundles/cros/usbc"
	_ "go.chromium.org/chromiumos/tast-tests/remote/bundles/cros/wifi"
	_ "go.chromium.org/chromiumos/tast-tests/remote/bundles/cros/wilco"

	_ "go.chromium.org/chromiumos/tast-tests/remote/bundles/cros/factory/fixture"
	_ "go.chromium.org/chromiumos/tast-tests/remote/meta" // import fixture for meta tests
)
