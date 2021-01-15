// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"chromiumos/tast/local/chrome/internal/driver"
)

// Conn represents a connection to a web content view, e.g. a tab.
type Conn = driver.Conn

// JSObject is a reference to a JavaScript object.
// JSObjects must be released or they will stop the JavaScript GC from freeing the memory they reference.
type JSObject = driver.JSObject
