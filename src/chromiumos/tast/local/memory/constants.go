// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memory

const (
	// KiB is the number of bytes in a kibibyte.
	KiB = 1024
	// MiB is the number of bytes in a mebibyte.
	MiB = KiB * 1024
	// GiB is the number of bytes in a gibibyte.
	GiB = MiB * 1024
	// TiB is the number of bytes in a tebibyte.
	TiB = GiB * 1024
	// PageBytes is the number of bytes in a page.
	PageBytes = 4096
	// KiBInMiB is the denominator to convert KiB to MiB
	KiBInMiB = 1024
)
