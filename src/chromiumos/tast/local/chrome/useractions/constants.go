// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package useractions

// AttributeTestScenario describes the test scenario that the user action is running in.
const AttributeTestScenario string = "TestScenario"

// AttributeFeature describes the feature that the user action is using.
const AttributeFeature string = "Feature"

// Attribute keys used to represent DUT environment.
const (
	AttributeDeviceMode    string = "DeviceMode"
	AttributeDeviceRegion  string = "DeviceRegion"
	AttributeKeyboardType  string = "KeyboardType"
	AttributeBoardName     string = "BoardName"
	AttributeIncognitoMode string = "IncognitoMode"
	AttributeUserMode      string = "UserMode"
	AttributeInputMethod   string = "InputMethod"
	AttributeInputField    string = "InputField"
	AttributeFloatVK       string = "FloatVK"
)

// Available attribute values of device mode.
const (
	DeviceModeClamshell string = "Clamshell"
	DeviceModeTablet    string = "Tablet"
	DeviceModeUnknown   string = "Unknown"
)

// Available attribute values of keyboard type.
const (
	KeyboardTypePhysicalKeyboard string = "Physical Keyboard"
	KeyboardTypeTabletVK         string = "Tablet Virtual Keyboard"
	KeyboardTypeA11yVK           string = "A11y Virtual Keyboard"
	KeyboardTypeUnknown          string = "Unknown"
)

// ActionTag is a string type to represent tag type of UserAction.
type ActionTag string

// Action tags to indicate interested products / teams.
const (
	ActionTagEssentialInputs ActionTag = "Essential Inputs"
	ActionTagARC             ActionTag = "ARC++"
	ActionTagOSSettings      ActionTag = "OS Settings"
	ActionTagIMESettings     ActionTag = "IME Settings"
	ActionTagIMEShelf        ActionTag = "IME Shelf"
)

// E14s feature definition.
const (
	FeatureIMEManagement      string = "IME Management"
	FeatureIMESpecific        string = "IME Specific Feature"
	FeaturePKTyping           string = "PK Typing Input"
	FeatureDeadKeys           string = "Dead Keys"
	FeatureVKTyping           string = "VK Typing Input"
	FeatureVKAutoShift        string = "VK AutoShift"
	FeatureVoiceInput         string = "Voice Input"
	FeatureHandWriting        string = "Handwriting"
	FeatureFloatVK            string = "Float VK"
	FeatureGlideTyping        string = "Glide Typing"
	FeatureEmoji              string = "Emoji"
	FeatureEmojiPicker        string = "Emoji Picker"
	FeatureEmojiSuggestion    string = "Emoji Suggestion"
	FeatureGrammarCheck       string = "Grammar Check"
	FeatureMultiPaste         string = "Multi-Paste"
	FeatureAutoCorrection     string = "Auto-Correction"
	FeatureAutoCapitalization string = "Auto-Capitalization"
)
