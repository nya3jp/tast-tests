// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arping

import (
	"regexp"
)

var (
	// unicastRE regexp for unicast reply/request.
	unicastRE = regexp.MustCompile(`(?m)^Unicast (reply|request) from ` +
		`(\d{1,3}(?:\.\d{1,3}){3}) \[([0-9A-F]{2}(?::[0-9A-F]{2}){5})\]  (\d+(?:\.\d+)?)ms$`)
	// sentRE regexp for sent probes.
	sentRE = regexp.MustCompile(`(?m)^Sent (\d+) probes`)
	// receivedRE regexp for received responses.
	receivedRE = regexp.MustCompile(`(?m)^Received (\d+) response\(s\)`)
)
