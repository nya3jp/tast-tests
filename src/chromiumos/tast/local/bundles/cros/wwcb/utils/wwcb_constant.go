// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import "time"

// Timeout for verifying audio, ethernet, display, power with dock
const (
	AudioTimeout    = 30 * time.Second
	EthernetTimeout = 30 * time.Second
	DisplayTimeout  = 30 * time.Second
	PowerTimeout    = 30 * time.Second
)
