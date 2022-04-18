// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package diagnosticsapp contains drivers for controlling the ui of diagnostics SWA.
package diagnosticsapp

import (
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
)

// regionalKeys defines keys which specified by region.
var regionalKeys = map[string][]string{
	"us": {"esc", "backspace", "shift", "alt", "ctrl"},
	"jp": {"あ", "ほ", "ゆ", "英数", "かな"},
	"fr": {"échap", "é", "ù", "◌̂", "alt gr"},
}

// DxInternalKeyboardTestButtons defines test button for internal keyboard which specified by region.
var DxInternalKeyboardTestButtons = map[string]*nodewith.Finder{
	"us": DxInternalKeyboardTestButton,
	"jp": nodewith.Name("テスト").Role(role.Button).First(),
	"fr": nodewith.Name("Tester").Role(role.Button).First(),
}
