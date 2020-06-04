// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package vmpstoredump provides constants and utilities to test vm_pstore_dump command.
package vmpstoredump

const (
	// VMPstoreDumpPath is the path to vm_pstore_dump command.
	VMPstoreDumpPath   = "/usr/bin/vm_pstore_dump"
	ramoopsConsoleSize = 0x40000
	// ConsoleBufferSize is the expected size of the ring buffer for the console output. The size is ramoops.console_size - persistent_ram_buffer.sig (4 byte) - persistent_ram_buffer.start (4 byte) - persistent_ram_buffer.size (4 byte)
	ConsoleBufferSize = ramoopsConsoleSize - 12
)
