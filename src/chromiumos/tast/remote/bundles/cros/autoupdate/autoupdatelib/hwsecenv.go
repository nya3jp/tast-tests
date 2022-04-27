// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package autoupdatelib provides common utils for making autoupdate tests.
package autoupdatelib

import (
	"chromiumos/tast/common/hwsec"
	hwsecremote "chromiumos/tast/remote/hwsec"
)

// HwsecEnv groups all hwsec objects together for more convenient access.
type HwsecEnv struct {
	CmdRunner *hwsecremote.CmdRunnerRemote
	Helper    *hwsecremote.CmdHelperRemote
	Utility   *hwsec.CryptohomeClient
}
