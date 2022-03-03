// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"time"
)

// Timeout & interval for verifying audio, ethernet, display, power status when a dock interacts with Chromebook.
const (
	AudioTimeout  = 30 * time.Second
	AudioInterval = 200 * time.Millisecond

	EthernetTimeout  = 30 * time.Second
	EthernetInterval = 200 * time.Millisecond

	DisplayTimeout  = 30 * time.Second
	DisplayInterval = 200 * time.Millisecond

	PowerTimeout  = 30 * time.Second
	PowerInterval = 200 * time.Millisecond
)

// Timeout & interval for verify windows fitting to certain properties.
const (
	WindowTimeout  = 30 * time.Second
	WindowInterval = 200 * time.Millisecond
)
