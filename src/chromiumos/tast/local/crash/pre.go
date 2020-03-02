// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// ChromePreWithVerboseConsent returns a precondition that will start chrome with the ChromeVerboseConsentFlags.
func ChromePreWithVerboseConsent() testing.Precondition {
	return chrome.NewPrecondition("verbose_logged_in", chrome.ExtraArgs(ChromeVerboseConsentFlags))
}
