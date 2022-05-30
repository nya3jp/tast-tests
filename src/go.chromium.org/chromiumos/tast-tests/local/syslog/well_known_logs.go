// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package syslog

const (
	// MessageFile is the name of main system log.
	MessageFile = "/var/log/messages"

	// ChromeLogFile is a symlink to the current Chrome log.
	ChromeLogFile = "/var/log/chrome/chrome"

	// NetLogFile is the name of network log.
	NetLogFile = "/var/log/net.log"
)
