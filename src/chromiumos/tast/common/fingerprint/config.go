// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fingerprint

// Possible fingerprint cros-config enum values can be seen at the following:
// source.chromium.org/chromium/chromiumos/platform2/+/main:chromeos-config/cros_config_host/cros_config_schema.yaml

// FPBoardName is the board name of the FPMCU. This is also the cros-config
// fingerprint board value.
type FPBoardName string

// Possible names for FPMCUs.
const (
	FPBoardNameBloonchipper FPBoardName = "bloonchipper"
	FPBoardNameDartmonkey   FPBoardName = "dartmonkey"
	FPBoardNameNocturne     FPBoardName = "nocturne_fp"
	FPBoardNameNami         FPBoardName = "nami_fp"
)
