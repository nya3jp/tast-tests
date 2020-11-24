// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package main implements the "cros" local test bundle.
//
// This executable contains standard Chrome OS tests.
package main

import (
	"chromiumos/tast/local/bundlemain"
	// Underscore-imported packages register their tests via init functions.
	_ "chromiumos/tast/local/bundles/cros/ad"
	_ "chromiumos/tast/local/bundles/cros/ambient"
	_ "chromiumos/tast/local/bundles/cros/apps"
	_ "chromiumos/tast/local/bundles/cros/arc"
	_ "chromiumos/tast/local/bundles/cros/arcappcompat"
	_ "chromiumos/tast/local/bundles/cros/assistant"
	_ "chromiumos/tast/local/bundles/cros/audio"
	_ "chromiumos/tast/local/bundles/cros/audio/soundcardinit"
	_ "chromiumos/tast/local/bundles/cros/baserpc"
	_ "chromiumos/tast/local/bundles/cros/biod"
	_ "chromiumos/tast/local/bundles/cros/camera"
	_ "chromiumos/tast/local/bundles/cros/crash"
	_ "chromiumos/tast/local/bundles/cros/crostini"
	_ "chromiumos/tast/local/bundles/cros/cryptohome"
	_ "chromiumos/tast/local/bundles/cros/dbus"
	_ "chromiumos/tast/local/bundles/cros/debugd"
	_ "chromiumos/tast/local/bundles/cros/dev"
	_ "chromiumos/tast/local/bundles/cros/documentscanapi"
	_ "chromiumos/tast/local/bundles/cros/enterprise"
	_ "chromiumos/tast/local/bundles/cros/example"
	_ "chromiumos/tast/local/bundles/cros/factory"
	_ "chromiumos/tast/local/bundles/cros/feedback"
	_ "chromiumos/tast/local/bundles/cros/filemanager"
	_ "chromiumos/tast/local/bundles/cros/firmware"
	_ "chromiumos/tast/local/bundles/cros/gamepad"
	_ "chromiumos/tast/local/bundles/cros/graphics"
	_ "chromiumos/tast/local/bundles/cros/hardware"
	_ "chromiumos/tast/local/bundles/cros/health"
	_ "chromiumos/tast/local/bundles/cros/hwsec"
	_ "chromiumos/tast/local/bundles/cros/inputs"
	_ "chromiumos/tast/local/bundles/cros/kernel"
	_ "chromiumos/tast/local/bundles/cros/lacros"
	_ "chromiumos/tast/local/bundles/cros/launcher"
	_ "chromiumos/tast/local/bundles/cros/logs"
	_ "chromiumos/tast/local/bundles/cros/meta"
	_ "chromiumos/tast/local/bundles/cros/multivm"
	_ "chromiumos/tast/local/bundles/cros/nacl"
	_ "chromiumos/tast/local/bundles/cros/nearbyshare"
	_ "chromiumos/tast/local/bundles/cros/network"
	_ "chromiumos/tast/local/bundles/cros/notifications"
	_ "chromiumos/tast/local/bundles/cros/ocr"
	_ "chromiumos/tast/local/bundles/cros/peripherals"
	_ "chromiumos/tast/local/bundles/cros/platform"
	_ "chromiumos/tast/local/bundles/cros/policy"
	_ "chromiumos/tast/local/bundles/cros/power"
	_ "chromiumos/tast/local/bundles/cros/printer"
	_ "chromiumos/tast/local/bundles/cros/qemu"
	_ "chromiumos/tast/local/bundles/cros/scanapp"
	_ "chromiumos/tast/local/bundles/cros/scanner"
	_ "chromiumos/tast/local/bundles/cros/sched"
	_ "chromiumos/tast/local/bundles/cros/security"
	_ "chromiumos/tast/local/bundles/cros/session"
	_ "chromiumos/tast/local/bundles/cros/shelf"
	_ "chromiumos/tast/local/bundles/cros/storage"
	_ "chromiumos/tast/local/bundles/cros/u2fd"
	_ "chromiumos/tast/local/bundles/cros/ui"
	_ "chromiumos/tast/local/bundles/cros/unicorn"
	_ "chromiumos/tast/local/bundles/cros/video"
	_ "chromiumos/tast/local/bundles/cros/vm"
	_ "chromiumos/tast/local/bundles/cros/webrtc"
	_ "chromiumos/tast/local/bundles/cros/wilco"
)

func main() {
	bundlemain.RunLocal()
}
