// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fingerprint

// FirmwareFilePath is the directory that hold fingerprint MCU firmware files.
const FirmwareFilePath = "/opt/google/biod/fw"

// FirmwareFilePattern produces a glob pattern for the specific fingerprint
// MCU firmware on rootfs. Note, some devices might contain multiple firmware
// files for different fingerprint variants but this tool will only yield
// the file for the active FPMCU.
func FirmwareFilePattern(board BoardName) string {
	return FirmwareFilePath + "/" + string(board) + "*.bin"
}
