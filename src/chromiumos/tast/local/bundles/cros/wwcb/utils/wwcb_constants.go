// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import "time"

// Timeout for verifying audio, ethernet, display, power status
// when dock interact with Chromebook
const (
	AudioTimeout    = 30 * time.Second
	EthernetTimeout = 30 * time.Second
	DisplayTimeout  = 30 * time.Second
	PowerTimeout    = 30 * time.Second
)

// Timeout for verify windows fitting to certain property
const (
	WindowTimeout = 30 * time.Second
)
