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
	PreviousObject   = "Search+Left"
	PreviousTab      = "Ctrl+Shift+Tab"
	Space            = "Space"
)

// OpenOptionsPage is the run of keyboard shortcuts used to open the ChromeVox options page.
var OpenOptionsPage = []string{
	"Search+O",
	"O",
}

// VoiceData contains context about voice, language, and TTS engine data to use for a given ChromeVox instance.
type VoiceData struct {
	VoiceData  a11y.VoiceData
	EngineData a11y.TTSEngineData
}
