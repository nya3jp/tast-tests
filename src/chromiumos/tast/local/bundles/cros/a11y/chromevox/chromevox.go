// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package chromevox provides functions to assist with interacting with ChromeVox, the built in screenreader.
package chromevox

import (
	"chromiumos/tast/local/a11y"
)

// Constants for keyboard shortcuts.
const (
	Activate         = "Search+Space"
	ArrowDown        = "Down"
	Escape           = "Esc"
	Find             = "Ctrl+F"
	JumpToLauncher   = "Alt+Shift+L"
	JumpToStatusTray = "Alt+Shift+S"
	NextObject       = "Search+Right"
	OpenOptionsOne   = "Search+O"
	OpenOptionsTwo   = "O"
	PreviousObject   = "Search+Left"
	Space            = "Space"
)

// TestVoiceData contains context about voice, language, and TTS engine data to use for a given ChromeVox instance.
type TestVoiceData struct {
	VoiceData  a11y.VoiceData
	EngineData a11y.TTSEngineData
}
