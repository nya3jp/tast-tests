// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package regexps contains the compiled regexps used by ping package.
package regexps

import (
	"regexp"
)

var (
	// SentRE regexp for the sent packets.
	SentRE = regexp.MustCompile(`(\d+) packets transmitted`)
	// ReceivedRE regexp for the received packets.
	ReceivedRE = regexp.MustCompile(`(\d+) received`)
	// LossRE regexp for the lost packets.
	LossRE = regexp.MustCompile(`(\d+(?:\.\d+)?)% packet loss`)
	// StatRE regexp for ping command stats.
	StatRE = regexp.MustCompile(`(?:round-trip|rtt) min[^=]*= ` +
		`(\d+(?:\.\d+)?)/(\d+(?:\.\d+)?)/` +
		`(\d+(?:\.\d+)?)/(\d+(?:\.\d+)?)`)
)
