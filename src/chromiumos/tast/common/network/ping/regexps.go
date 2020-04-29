// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ping

import (
	"regexp"
)

var (
	// sentRE regexp for the sent packets.
	sentRE = regexp.MustCompile(`(\d+) packets transmitted`)
	// receivedRE regexp for the received packets.
	receivedRE = regexp.MustCompile(`(\d+) received`)
	// lossRE regexp for the lost packets.
	lossRE = regexp.MustCompile(`(\d+(?:\.\d+)?)% packet loss`)
	// statRE regexp for ping command stats.
	statRE = regexp.MustCompile(`(?:round-trip|rtt) min[^=]*= ` +
		`(\d+(?:\.\d+)?)/(\d+(?:\.\d+)?)/` +
		`(\d+(?:\.\d+)?)/(\d+(?:\.\d+)?)`)
)
