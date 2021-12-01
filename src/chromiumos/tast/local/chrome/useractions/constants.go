// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package useractions

// AttributeTestScenario describes the test scenario that the user action is running in.
const AttributeTestScenario string = "TestScenario"

// Attribute keys used to represent DUT environment.
const (
	AttributeDeviceMode    string = "DeviceMode"
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
	ActionTagEssentialInputs ActionTag = "EssentialInputs"
)

// Action tags to indicate components of the user action.
const (
	ActionTagOSSettings    ActionTag = "OSSettings"
	ActionTagIMEManagement ActionTag = "IMEManagement"
	ActionTagIMESettings   ActionTag = "IMESettings"
)

// Action tags to indicate actions in IME Management.
const (
	ActionTagAddIME    ActionTag = "AddInputMethod"
	ActionTagRemoveIME ActionTag = "RemoveInputMethod"
	ActionTagSwitchIME ActionTag = "SwitchIME"
)

// Tags to indicate user input actions.
const (
	ActionTagPKTyping        ActionTag = "PKTyping"
	ActionTagDeadKey         ActionTag = "DeadKey"
	ActionTagVKTyping        ActionTag = "VKTyping"
	ActionTagVKVoiceInput    ActionTag = "VKVoiceInput"
	ActionTagVKHandWriting   ActionTag = "VKHandWriting"
	ActionTagGlideTyping     ActionTag = "GlideTyping"
	ActionTagEmoji           ActionTag = "Emoji"
	ActionTagEmojiPicker     ActionTag = "EmojiPicker"
	ActionTagEmojiSuggestion ActionTag = "EmojiSuggestion"
	ActionTagMultiPaste      ActionTag = "MultiPaste"
)
