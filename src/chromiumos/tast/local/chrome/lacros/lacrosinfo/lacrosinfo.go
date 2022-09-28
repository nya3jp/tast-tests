// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lacrosinfo allows querying Ash about the state of Lacros
package lacrosinfo

import (
	lacros "chromiumos/tast/local/chrome/internal/lacros"
)

// LacrosState represents the state of Lacros. See BrowserManager::State in Chromium's browser_manager.h.
type LacrosState = lacros.LacrosState

// LacrosState values. To be extended on demand.
const (
	LacrosStateStopped LacrosState = lacros.LacrosStateStopped
)

// LacrosMode represents the mode of Lacros. See crosapi::browser_util::LacrosMode.
type LacrosMode = lacros.LacrosMode

// LacrosMode values.
const (
	LacrosModeDisabled   LacrosMode = lacros.LacrosModeDisabled
	LacrosModeSideBySide LacrosMode = lacros.LacrosModeSideBySide
	LacrosModePrimary    LacrosMode = lacros.LacrosModePrimary
	LacrosModeOnly       LacrosMode = lacros.LacrosModeOnly
)

// Info represents the format returned from autotestPrivate.getLacrosInfo.
type Info = lacros.Info

// Snapshot gets the current lacros info from ash-chrome. The parameter tconn should be the ash TestConn.
var Snapshot = lacros.Snapshot
