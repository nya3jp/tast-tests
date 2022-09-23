// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package demomode

import (
	"chromiumos/tast/local/chrome"
)

// Context contains data / connections to be passed from Demo Mode fixture to tests
type Context struct {
	Chrome *chrome.Chrome
	Tconn  *chrome.TestConn
}
