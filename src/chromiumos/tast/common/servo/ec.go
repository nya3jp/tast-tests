// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package servo

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// These are the EC Servo controls which can be get/set with a string value.
const (
	ECBoard            StringControl = "ec_board"
	ECSystemPowerState StringControl = "ec_system_powerstate"
	ECUARTCmd          StringControl = "ec_uart_cmd"
	ECUARTRegexp       StringControl = "ec_uart_regexp"
	ECUARTStream       StringControl = "ec_uart_stream"
	ECChip             StringControl = "ec_chip"
	ECFlashSize        StringControl = "ec_flash_size"
	DUTPDDataRole      StringControl = "dut_pd_data_role"
)

// These controls accept only "on" and "off" as values.
const (
	ECUARTCapture OnOffControl = "ec_uart_capture"
)

// Cmd constants for RunECCommand.
const (
	// Using with no additional arguments returns current backlight level
	// If additional int arg (0-100) provided, sets backlight to that level
	kbLight string = "kblight"
)

// Pattern expression for RunCommandGetOutput.
const (
	reKBBacklight        string = `Keyboard backlight: (\d+)\%`
	reCheckKBLight       string = `Keyboard backlight: \d+\%|Command 'kblight' not found or ambiguous.`
	reTabletmodeNotFound string = `Command 'tabletmode' not found or ambiguous`
	reBasestateNotFound  string = `Command 'basestate' not found or ambiguous`
	reTabletmodeStatus   string = `\[\S+ tablet mode (enabled|disabled)\]`
	reBasestateStatus    string = `\[\S+ base state: (attached|detached)\]`
	reBdStatus           string = `\[\S+ BD forced (connected|disconnected)\]`
	reLidAccel           string = `\[\S+ Lid Accel ODR:(?i)[^\n\r]*(?i)(1|0)\S+]`
)

// USBCDataRole is a USB-C data role.
type USBCDataRole string

// USB-C data roles.
const (
	// UFP is Upward facing partner, i.e. a peripheral. The servo should normally be in this role.
	UFP USBCDataRole = "UFP"
	// DFP is Downward facing partner, i.e. a host. The DUT should normally be in this role.
	DFP USBCDataRole = "DFP"
)

// KBMatrixPair is a struct to store key row and col for the kbpress cmd
type KBMatrixPair struct {
	row int
	col int
}

// KeyMatrix is a map that stores a row/col pair for each key using KBMatrixPair
// It's stored in order of appearance in a keyboard
var KeyMatrix = map[string]KBMatrixPair{
	"<esc>":       KBMatrixPair{1, 1},
	"<f1>":        KBMatrixPair{0, 2},
	"<f2>":        KBMatrixPair{3, 2},
	"<f3>":        KBMatrixPair{2, 2},
	"<f4>":        KBMatrixPair{1, 2},
	"<f5>":        KBMatrixPair{3, 4},
	"<f6>":        KBMatrixPair{2, 4},
	"<f7>":        KBMatrixPair{1, 4},
	"<f8>":        KBMatrixPair{2, 9},
	"<f9>":        KBMatrixPair{1, 9},
	"<f10>":       KBMatrixPair{0, 4},
	"`":           KBMatrixPair{3, 1},
	"1":           KBMatrixPair{6, 1},
	"2":           KBMatrixPair{6, 4},
	"3":           KBMatrixPair{6, 2},
	"4":           KBMatrixPair{6, 3},
	"5":           KBMatrixPair{3, 3},
	"6":           KBMatrixPair{3, 6},
	"7":           KBMatrixPair{6, 6},
	"8":           KBMatrixPair{6, 5},
	"9":           KBMatrixPair{6, 9},
	"0":           KBMatrixPair{6, 8},
	"-":           KBMatrixPair{3, 8},
	"=":           KBMatrixPair{0, 8},
	"<backspace>": KBMatrixPair{1, 11},
	"<tab>":       KBMatrixPair{2, 1},
	"q":           KBMatrixPair{7, 1},
	"w":           KBMatrixPair{7, 4},
	"e":           KBMatrixPair{7, 2},
	"r":           KBMatrixPair{7, 3},
	"t":           KBMatrixPair{2, 3},
	"y":           KBMatrixPair{2, 6},
	"u":           KBMatrixPair{7, 6},
	"i":           KBMatrixPair{7, 5},
	"o":           KBMatrixPair{7, 9},
	"p":           KBMatrixPair{7, 8},
	"[":           KBMatrixPair{2, 8},
	"]":           KBMatrixPair{2, 5},
	"\\":          KBMatrixPair{3, 11},
	"<search>":    KBMatrixPair{0, 1},
	"a":           KBMatrixPair{4, 1},
	"s":           KBMatrixPair{4, 4},
	"d":           KBMatrixPair{4, 2},
	"f":           KBMatrixPair{4, 3},
	"g":           KBMatrixPair{1, 3},
	"h":           KBMatrixPair{1, 6},
	"j":           KBMatrixPair{4, 6},
	"k":           KBMatrixPair{4, 5},
	"l":           KBMatrixPair{4, 9},
	";":           KBMatrixPair{4, 8},
	"'":           KBMatrixPair{1, 8},
	"<enter>":     KBMatrixPair{4, 11},
	"<shift_l>":   KBMatrixPair{5, 7},
	"z":           KBMatrixPair{5, 1},
	"x":           KBMatrixPair{5, 4},
	"c":           KBMatrixPair{5, 2},
	"v":           KBMatrixPair{5, 3},
	"b":           KBMatrixPair{0, 3},
	"n":           KBMatrixPair{0, 6},
	"m":           KBMatrixPair{5, 6},
	",":           KBMatrixPair{5, 5},
	".":           KBMatrixPair{5, 9},
	"/":           KBMatrixPair{5, 8},
	"<shift_r>":   KBMatrixPair{7, 7},
	"<ctrl_l>":    KBMatrixPair{2, 0},
	"<alt_l>":     KBMatrixPair{6, 10},
	" ":           KBMatrixPair{5, 1},
	"<alt_r>":     KBMatrixPair{0, 10},
	"<ctrl_r>":    KBMatrixPair{4, 0},
	"<left>":      KBMatrixPair{7, 12},
	"<up>":        KBMatrixPair{7, 11},
	"<down>":      KBMatrixPair{6, 11},
	"<right>":     KBMatrixPair{6, 12},
}

// KeyCode holds int codes for key events.
type KeyCode int

// The linux key codes representing key event.
const (
	KeyReserved   KeyCode = 0
	KeyEsc        KeyCode = 1
	Key1          KeyCode = 2
	Key2          KeyCode = 3
	Key3          KeyCode = 4
	Key4          KeyCode = 5
	Key5          KeyCode = 6
	Key6          KeyCode = 7
	Key7          KeyCode = 8
	Key8          KeyCode = 9
	Key9          KeyCode = 10
	Key0          KeyCode = 11
	KeyMinus      KeyCode = 12
	KeyEqual      KeyCode = 13
	KeyBackspace  KeyCode = 14
	KeyTab        KeyCode = 15
	KeyQ          KeyCode = 16
	KeyW          KeyCode = 17
	KeyE          KeyCode = 18
	KeyR          KeyCode = 19
	KeyT          KeyCode = 20
	KeyY          KeyCode = 21
	KeyU          KeyCode = 22
	KeyI          KeyCode = 23
	KeyO          KeyCode = 24
	KeyP          KeyCode = 25
	KeyLeftBrace  KeyCode = 26
	KeyRightBrace KeyCode = 27
	KeyEnter      KeyCode = 28
	KeyLeftCtrl   KeyCode = 29
	KeyA          KeyCode = 30
	KeyS          KeyCode = 31
	KeyD          KeyCode = 32
	KeyF          KeyCode = 33
	KeyG          KeyCode = 34
	KeyH          KeyCode = 35
	KeyJ          KeyCode = 36
	KeyK          KeyCode = 37
	KeyL          KeyCode = 38
	KeySemicolon  KeyCode = 39
	KeyApostrophe KeyCode = 40
	KeyGrave      KeyCode = 41
	KeyLeftShift  KeyCode = 42
	KeyBackslash  KeyCode = 43
	KeyZ          KeyCode = 44
	KeyX          KeyCode = 45
	KeyC          KeyCode = 46
	KeyV          KeyCode = 47
	KeyB          KeyCode = 48
	KeyN          KeyCode = 49
	KeyM          KeyCode = 50
	KeyComma      KeyCode = 51
	KeyDot        KeyCode = 52
	KeySlash      KeyCode = 53
	KeyRightShift KeyCode = 54
	KeyKPAsterisk KeyCode = 55
	KeyLeftAlt    KeyCode = 56
	KeySpace      KeyCode = 57
	KeyCapslock   KeyCode = 58
	KeyF1         KeyCode = 59
	KeyF2         KeyCode = 60
	KeyF3         KeyCode = 61
	KeyF4         KeyCode = 62
	KeyF5         KeyCode = 63
	KeyF6         KeyCode = 64
	KeyF7         KeyCode = 65
	KeyF8         KeyCode = 66
	KeyF9         KeyCode = 67
	KeyF10        KeyCode = 68
	KeyNumlock    KeyCode = 69
	KeyScrolllock KeyCode = 70
	KeyKP7        KeyCode = 71
	KeyKP8        KeyCode = 72
	KeyKP9        KeyCode = 73
	KeyKPMinus    KeyCode = 74
	KeyKP4        KeyCode = 75
	KeyKP5        KeyCode = 76
	KeyKP6        KeyCode = 77
	KeyKPPlus     KeyCode = 78
	KeyKP1        KeyCode = 79
	KeyKP2        KeyCode = 80
	KeyKP3        KeyCode = 81
	KeyKP0        KeyCode = 82
	KeyKPDot      KeyCode = 83

	KeyZenkakuhankaku   KeyCode = 85
	Key102nd            KeyCode = 86
	KeyF11              KeyCode = 87
	KeyF12              KeyCode = 88
	KeyRo               KeyCode = 89
	KeyKatakana         KeyCode = 90
	KeyHiragana         KeyCode = 91
	KeyHenkan           KeyCode = 92
	KeyKatakanahiragana KeyCode = 93
	KeyMuhenkan         KeyCode = 94
	KeyKPJpComma        KeyCode = 95
	KeyKPEnter          KeyCode = 96
	KeyRightCtrl        KeyCode = 97
	KeyKPSlash          KeyCode = 98
	KeySysrq            KeyCode = 99
	KeyRightAlt         KeyCode = 100
	KeyLinefeed         KeyCode = 101
	KeyHome             KeyCode = 102
	KeyUp               KeyCode = 103
	KeyPageUp           KeyCode = 104
	KeyLeft             KeyCode = 105
	KeyRight            KeyCode = 106
	KeyEnd              KeyCode = 107
	KeyDown             KeyCode = 108
	KeyPageDown         KeyCode = 109
	KeyInsert           KeyCode = 110
	KeyDelete           KeyCode = 111
	KeyMacro            KeyCode = 112
	KeyMute             KeyCode = 113
	KeyVolumeDown       KeyCode = 114
	KeyVolumeUp         KeyCode = 115
	KeyPower            KeyCode = 116 // SC System Power Down
	KeyKPEqual          KeyCode = 117
	KeyKPPlusMinus      KeyCode = 118
	KeyPause            KeyCode = 119
	KeyScale            KeyCode = 120 // AL Compiz Scale (Expose)

	KeyKPComma   KeyCode = 121
	KeyHangeul   KeyCode = 122
	KeyHanguel   KeyCode = KeyHangeul
	KeyHanja     KeyCode = 123
	KeyYen       KeyCode = 124
	KeyLeftMeta  KeyCode = 125
	KeyRightMeta KeyCode = 126
	KeyCompose   KeyCode = 127

	KeyStop         KeyCode = 128 // AC Stop
	KeyAgain        KeyCode = 129
	KeyProps        KeyCode = 130 // AC Properties
	KeyUndo         KeyCode = 131 // AC Undo
	KeyFront        KeyCode = 132
	KeyCopy         KeyCode = 133 // AC Copy
	KeyOpen         KeyCode = 134 // AC Open
	KeyPaste        KeyCode = 135 // AC Paste
	KeyFind         KeyCode = 136 // AC Search
	KeyCut          KeyCode = 137 // AC Cut
	KeyHelp         KeyCode = 138 // AL Integrated Help Center
	KeyMenu         KeyCode = 139 // Menu (show menu)
	KeyCalc         KeyCode = 140 // AL Calculator
	KeySetup        KeyCode = 141
	KeySleep        KeyCode = 142 // SC System Sleep
	KeyWakeUp       KeyCode = 143 // System Wake Up
	KeyFile         KeyCode = 144 // AL Local Machine Browser
	KeySendFile     KeyCode = 145
	KeyDeleteFile   KeyCode = 146
	KeyXfer         KeyCode = 147
	KeyProg1        KeyCode = 148
	KeyProg2        KeyCode = 149
	KeyWww          KeyCode = 150 // AL Internet Browser
	KeyMsdos        KeyCode = 151
	KeyCoffee       KeyCode = 152 // AL Terminal Lock/Screensaver
	KeyScreenlock   KeyCode = KeyCoffee
	KeyDirection    KeyCode = 153
	KeyCycleWindows KeyCode = 154
	KeyMail         KeyCode = 155
	KeyBookmarks    KeyCode = 156 // AC Bookmarks
	KeyComputer     KeyCode = 157
	KeyBack         KeyCode = 158 // AC Back
	KeyForward      KeyCode = 159 // AC Forward
	KeyCloseCD      KeyCode = 160
	KeyEjectCD      KeyCode = 161
	KeyEjectClosecd KeyCode = 162
	KeyNextSong     KeyCode = 163
	KeyPlayPause    KeyCode = 164
	KeyPreviousSong KeyCode = 165
	KeyStopCD       KeyCode = 166
	KeyRecord       KeyCode = 167
	KeyRewind       KeyCode = 168
	KeyPhone        KeyCode = 169 // Media Select Telephone
	KeyIso          KeyCode = 170
	KeyConfig       KeyCode = 171 // AL Consumer Control Configuration
	KeyHomepage     KeyCode = 172 // AC Home
	KeyRefresh      KeyCode = 173 // AC Refresh
	KeyExit         KeyCode = 174 // AC Exit
	KeyMove         KeyCode = 175
	KeyEdit         KeyCode = 176
	KeyScrollUp     KeyCode = 177
	KeyScrollDown   KeyCode = 178
	KeyKPLeftParen  KeyCode = 179
	KeyKPRightParen KeyCode = 180
	KeyNew          KeyCode = 181 // AC New
	KeyRedo         KeyCode = 182 // AC Redo/Repeat

	KeyF13 KeyCode = 183
	KeyF14 KeyCode = 184
	KeyF15 KeyCode = 185
	KeyF16 KeyCode = 186
	KeyF17 KeyCode = 187
	KeyF18 KeyCode = 188
	KeyF19 KeyCode = 189
	KeyF20 KeyCode = 190
	KeyF21 KeyCode = 191
	KeyF22 KeyCode = 192
	KeyF23 KeyCode = 193
	KeyF24 KeyCode = 194

	KeyPlayCD         KeyCode = 200
	KeyPauseCD        KeyCode = 201
	KeyProg3          KeyCode = 202
	KeyProg4          KeyCode = 203
	KeyDashboard      KeyCode = 204 // AL Dashboard
	KeySuspend        KeyCode = 205
	KeyClose          KeyCode = 206 // AC Close
	KeyPlay           KeyCode = 207
	KeyFastforward    KeyCode = 208
	KeyBassboost      KeyCode = 209
	KeyPrint          KeyCode = 210 // AC Print
	KeyHp             KeyCode = 211
	KeyCamera         KeyCode = 212
	KeySound          KeyCode = 213
	KeyQuestion       KeyCode = 214
	KeyEmail          KeyCode = 215
	KeyChat           KeyCode = 216
	KeySearch         KeyCode = 217
	KeyConnect        KeyCode = 218
	KeyFinance        KeyCode = 219 //AL Checkbook/Finance
	KeySport          KeyCode = 220
	KeyShop           KeyCode = 221
	KeyAlterase       KeyCode = 222
	KeyCancel         KeyCode = 223 // AC Cancel
	KeyBrightnessDown KeyCode = 224
	KeyBrightnessUp   KeyCode = 225
	KeyMedia          KeyCode = 226

	KeySwitchVideoMode KeyCode = 227 // Cycle between available video
	// outputs (Monitor/LCD/TV-out/etc)
	KeyKbdillumToggle KeyCode = 228
	KeyKbdillumDown   KeyCode = 229
	KeyKbdillumUp     KeyCode = 230

	KeySend        KeyCode = 231 // AC Send
	KeyReply       KeyCode = 232 // AC Reply
	KeyForwardMail KeyCode = 233 // AC Forward Msg
	KeySave        KeyCode = 234 // AC Save
	KeyDocuments   KeyCode = 235

	KeyBattery KeyCode = 236

	KeyBluetooth KeyCode = 237
	KeyWLAN      KeyCode = 238
	KeyUWB       KeyCode = 239

	KeyUnknown KeyCode = 240

	KeyVideoNext       KeyCode = 241 // drive next video source
	KeyVideoPrev       KeyCode = 242 // drive previous video source
	KeyBrightnessCycle KeyCode = 243 // brightness up, after max is min
	KeyBrightnessZero  KeyCode = 244 // brightness off, use ambient
	KeyDisplayOff      KeyCode = 245 // display device to off state

	KeyWimax  KeyCode = 246
	KeyRFKill KeyCode = 247 // Key that controls all radios
)

// HibernationOpt is an option for hibernating DUT.
type HibernationOpt string

// Available options for triggering hibernation.
const (
	// UseKeyboard uses keyboard shortcut for hibernating DUT: alt+vol_up+h.
	UseKeyboard HibernationOpt = "keyboard"
	// UseConsole uses the EC command `hibernate` to put DUT in hibernation.
	UseConsole HibernationOpt = "console"
)

// RunECCommand runs the given command on the EC on the device.
func (s *Servo) RunECCommand(ctx context.Context, cmd string) error {
	if err := s.SetString(ctx, ECUARTRegexp, "None"); err != nil {
		return errors.Wrap(err, "Clearing EC UART Regexp")
	}
	return s.SetString(ctx, ECUARTCmd, cmd)
}

// RunECCommandGetOutput runs the given command on the EC on the device and returns the output matching patterns.
func (s *Servo) RunECCommandGetOutput(ctx context.Context, cmd string, patterns []string) ([][]string, error) {
	err := s.SetStringList(ctx, ECUARTRegexp, patterns)
	if err != nil {
		return nil, errors.Wrapf(err, "setting ECUARTRegexp to %s", patterns)
	}
	defer s.SetString(ctx, ECUARTRegexp, "None")
	err = s.SetString(ctx, ECUARTCmd, cmd)
	if err != nil {
		return nil, errors.Wrapf(err, "setting ECUARTCmd to %s", cmd)
	}
	iList, err := s.GetStringList(ctx, ECUARTCmd)
	if err != nil {
		return nil, errors.Wrap(err, "decoding string list")
	}
	return ConvertToStringArrayArray(ctx, iList)
}

// GetECSystemPowerState returns the power state, like "S0" or "G3"
func (s *Servo) GetECSystemPowerState(ctx context.Context) (string, error) {
	return s.GetString(ctx, ECSystemPowerState)
}

// ECHibernate puts the EC into hibernation mode, after removing the servo watchdog for CCD if necessary.
func (s *Servo) ECHibernate(ctx context.Context, option HibernationOpt) error {
	if err := s.WatchdogRemove(ctx, WatchdogCCD); err != nil {
		return errors.Wrap(err, "failed to remove watchdog for ccd")
	}
	switch option {
	case "keyboard":
		if err := func(ctx context.Context) error {
			for _, targetKey := range []string{"<alt_l>", "<f10>", "h"} {
				row, col, err := s.GetKeyRowCol(targetKey)
				if err != nil {
					return errors.Wrapf(err, "failed to get key %s column and row", targetKey)
				}
				targetKeyName := targetKey
				targetKeyHold := fmt.Sprintf("kbpress %d %d 1", col, row)
				targetKeyRelease := fmt.Sprintf("kbpress %d %d 0", col, row)
				testing.ContextLogf(ctx, "Pressing and holding key %s", targetKey)
				if err := s.RunECCommand(ctx, targetKeyHold); err != nil {
					return errors.Wrapf(err, "failed to press and hold key %s", targetKey)
				}
				defer func(releaseKey, name string) error {
					testing.ContextLogf(ctx, "Releasing key %s", name)
					if err := s.RunECCommand(ctx, releaseKey); err != nil {
						return errors.Wrapf(err, "failed to release key %s", releaseKey)
					}
					return nil
				}(targetKeyRelease, targetKeyName)
			}
			return nil
		}(ctx); err != nil {
			return err
		}
	case "console":
		if err := s.RunECCommand(ctx, "hibernate"); err != nil {
			return errors.Wrap(err, "failed to run EC command: hibernate")
		}
	}

	// Delay for a few seconds to allow proper propagation of the
	// hibernation command, prior to checking EC unresponsive.
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		return errors.Wrap(err, "failed to sleep")
	}
	if err := s.CheckUnresponsiveEC(ctx); err != nil {
		return errors.Wrap(err, "while verifying whether EC is unresponsive after hibernating DUT")
	}
	return nil
}

// GetECFlashSize returns the size of EC in KB e.g. 512
func (s *Servo) GetECFlashSize(ctx context.Context) (int, error) {
	sizeStr, err := s.GetString(ctx, ECFlashSize)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get value for ec size")
	}
	// ECFlashSize method matches an int regex so Atoi should always work
	return strconv.Atoi(sizeStr)
}

// GetECChip returns the DUT chip e.g. "npcx_uut"
func (s *Servo) GetECChip(ctx context.Context) (string, error) {
	return s.GetString(ctx, ECChip)
}

// SetDUTPDDataRole tries to find the port attached to the servo, and performs a data role swap if the role doesn't match `role`.
// Will fail if there is no chromeos EC.
func (s *Servo) SetDUTPDDataRole(ctx context.Context, role USBCDataRole) error {
	return s.SetString(ctx, DUTPDDataRole, string(role))
}

// GetKeyRowCol returns the key row and column for kbpress cmd
func (s *Servo) GetKeyRowCol(key string) (int, int, error) {
	pair, ok := KeyMatrix[key]
	if !ok {
		return 0, 0, errors.New("failed to find key in KeyMatrix map")
	}
	return pair.row, pair.col, nil

}

// ECPressKey simulates a keypress on the DUT from the servo using kbpress.
func (s *Servo) ECPressKey(ctx context.Context, key string) error {
	row, col, err := s.GetKeyRowCol(key)
	if err != nil {
		return errors.Wrapf(err, "failed to get key %q in key matrix", key)
	}
	if err := s.RunECCommand(ctx, fmt.Sprintf("kbpress %d %d 1", col, row)); err != nil {
		return errors.Wrapf(err, "failed to press key %q", key)
	}
	if err := s.RunECCommand(ctx, fmt.Sprintf("kbpress %d %d 0", col, row)); err != nil {
		return errors.Wrapf(err, "failed to release key %q", key)
	}
	return nil
}

// SetKBBacklight sets the DUT keyboards backlight to the given value (0 - 100).
func (s *Servo) SetKBBacklight(ctx context.Context, percent int) error {
	testing.ContextLog(ctx, "Setting keyboard backlight to: ", percent)
	err := s.RunECCommand(ctx, fmt.Sprintf("%v %v", kbLight, percent))
	if err != nil {
		return errors.Wrapf(err, "running '%v %v' on DUT", kbLight, percent)
	}
	return nil
}

// GetKBBacklight gets the DUT keyboards backlight value in percent (0 - 100).
func (s *Servo) GetKBBacklight(ctx context.Context) (int, error) {
	testing.ContextLog(ctx, "Getting current keyboard backlight percent")
	out, err := s.RunECCommandGetOutput(ctx, kbLight, []string{reKBBacklight})
	if err != nil {
		return 0, errors.Wrapf(err, "running %v on DUT", kbLight)
	}
	return strconv.Atoi(out[0][1])
}

// HasKBBacklight checks if the DUT keyboards has backlight functionality.
func (s *Servo) HasKBBacklight(ctx context.Context) bool {
	testing.ContextLog(ctx, "Checking if DUT keyboard supports backlight")
	out, _ := s.RunECCommandGetOutput(ctx, kbLight, []string{reCheckKBLight})
	expMatch := regexp.MustCompile(reKBBacklight)
	match := expMatch.FindStringSubmatch(out[0][0])
	return match != nil
}

// CheckUnresponsiveEC verifies that EC console is unresponsive in situations such as
// hibernation and battery cutoff. Ignore null chars, sometimes the servo returns null
// when the EC is off.
func (s *Servo) CheckUnresponsiveEC(ctx context.Context) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		out, err := s.RunECCommandGetOutput(ctx, "version", []string{`[^\x00]+`})
		if err == nil {
			return errors.Errorf("EC is still active: got %v; expected error", out)
		}
		if !strings.Contains(err.Error(), "No data was sent from the pty") &&
			!strings.Contains(err.Error(), "EC: Timeout waiting for response.") &&
			!strings.Contains(err.Error(), "Timed out waiting for interfaces to become available") {
			return errors.Wrap(err, "unexpected EC error")
		}
		return nil
	}, &testing.PollOptions{Interval: 1 * time.Second, Timeout: 20 * time.Second})
}

// CheckAndRunTabletModeCommand checks if relevant EC commands exist and use them for setting tablet mode.
// For example, detachables use 'basestate (attach|detach)', and convertibles use 'tabletmode (on|off)'.
func (s *Servo) CheckAndRunTabletModeCommand(ctx context.Context, command string) (string, error) {
	// regular expressions.
	reStr := strings.Join([]string{reTabletmodeNotFound, reTabletmodeStatus,
		reBasestateNotFound, reBasestateStatus, reBdStatus, reLidAccel}, "|")
	checkTabletMode := fmt.Sprintf("%s%s%s", "(", reStr, ")")
	// Run EC command to check tablet mode setting.
	out, err := s.RunECCommandGetOutput(ctx, command, []string{checkTabletMode})
	if err != nil {
		return "", errors.Wrapf(err, "failed to run command %q", command)
	}
	tabletModeUnavailable := []*regexp.Regexp{regexp.MustCompile(reTabletmodeNotFound),
		regexp.MustCompile(reBasestateNotFound)}
	for _, v := range tabletModeUnavailable {
		if match := v.FindStringSubmatch(out[0][0]); match != nil {
			return "", errors.Errorf("device does not support tablet mode: %q", match)
		}
	}
	return string(out[0][1]), nil
}

// OpenCCD checks if a CCD connection exists, and then opens CCD if it's locked.
func (s *Servo) OpenCCD(ctx context.Context) error {
	if hasCCD, err := s.HasCCD(ctx); err != nil {
		return errors.Wrap(err, "while checking if servo has a CCD connection")
	} else if hasCCD {
		if val, err := s.GetString(ctx, GSCCCDLevel); err != nil {
			return errors.Wrap(err, "failed to get gsc_ccd_level")
		} else if val != Open {
			testing.ContextLogf(ctx, "CCD is not open, got %q. Attempting to unlock", val)
			if err := s.SetString(ctx, CR50Testlab, Open); err != nil {
				return errors.Wrap(err, "failed to unlock CCD")
			}
		}
		// For debugging purposes, log CCD state after unlocking CCD.
		checkedVal, err := s.GetString(ctx, GSCCCDLevel)
		if err != nil {
			return errors.Wrap(err, "failed to get gsc_ccd_level after unlocking CCD")
		}
		testing.ContextLogf(ctx, "CCD State: %q", checkedVal)
	}
	return nil
}
