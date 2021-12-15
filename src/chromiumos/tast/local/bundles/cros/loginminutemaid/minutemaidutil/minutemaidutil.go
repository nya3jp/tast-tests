// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package minutemaidutil

import (
	"chromiumos/tast/testing"
)

// Locale is a runtime variable, used by tests under loginminutemaid to set locale to en, cjk, rtl or verbose
var Locale = testing.RegisterVarString(
	"minutemaidutil.locale",
	"en",
	"The locale being passed to chrome.New()")
