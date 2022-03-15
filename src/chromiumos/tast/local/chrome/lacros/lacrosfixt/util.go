// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacrosfixt

// ExtensionArgs returns a list of args needed to pass to a lacros instance to enable the test extension.
func ExtensionArgs(extID, extList string) []string {
	return []string{
		"--remote-debugging-port=0",              // Let Chrome choose its own debugging port.
		"--enable-experimental-extension-apis",   // Allow Chrome to use the Chrome Automation API.
		"--allowlisted-extension-id=" + extID,    // Whitelists the test extension to access all Chrome APIs.
		"--load-extension=" + extList,            // Load extensions.
		"--disable-extensions-except=" + extList, // Disable extensions other than the Tast test extension.
	}
}
