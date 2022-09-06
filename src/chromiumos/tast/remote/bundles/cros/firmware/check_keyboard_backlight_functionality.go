// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	pb "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CheckKeyboardBacklightFunctionality,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Confirm keyboard backlight support and check keyboard backlight functionality",
		Contacts:     []string{"cienet-firmware@cienet.corp-partner.google.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.browser.ChromeService", "tast.cros.ui.ScreenRecorderService"},
		HardwareDeps: hwdep.D(
			hwdep.ChromeEC(),
			hwdep.KeyboardBacklight(),
		),
		Fixture: fixture.NormalMode,
	})
}

type timeoutError struct {
	*errors.E
}

// CheckKeyboardBacklightFunctionality confirms keyboard backlight support and verifies its functionality.
func CheckKeyboardBacklightFunctionality(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper

	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}
	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to get config: ", err)
	}

	// Perform a hard reset on DUT to ensure removal of any
	// old settings that might potentially have an impact on
	// this test.
	if err := h.Servo.SetPowerState(ctx, servo.PowerStateReset); err != nil {
		s.Fatal("Failed to cold reset DUT at the beginning of test: ", err)
	}

	if err := h.RequireRPCClient(ctx); err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}

	// Temporary sleep would help prevent the streaming RPC call error.
	s.Log("Sleeping for a few seconds before starting a new Chrome")
	if err := testing.Sleep(ctx, 5*time.Second); err != nil {
		s.Fatal("Failed to sleep for a few seconds: ", err)
	}

	s.Log("Starting a new Chrome")
	chromeRequest := pb.NewRequest{
		LoginMode: pb.LoginMode_LOGIN_MODE_GUEST_LOGIN,
	}
	chromeService := pb.NewChromeServiceClient(h.RPCClient.Conn)
	if _, err := chromeService.New(ctx, &chromeRequest); err != nil {
		s.Fatal("Failed to create new Chrome at login: ", err)
	}
	defer chromeService.Close(ctx, &empty.Empty{})

	s.Log("Screen recorder started")
	filePath := filepath.Join(s.OutDir(), "kblightRecord.webm")
	startRequest := pb.StartRequest{
		FileName: filePath,
	}
	screenRecorder := pb.NewScreenRecorderServiceClient(h.RPCClient.Conn)
	if _, err := screenRecorder.Start(ctx, &startRequest); err != nil {
		s.Fatal("Failed to start recording: ", err)
	}
	defer func() {
		res, err := screenRecorder.Stop(ctx, &empty.Empty{})
		if err != nil {
			s.Log("Unable to save the recording: ", err)
		} else {
			s.Logf("Screen recording saved to %s", res.FileName)
		}
	}()

	// Current hardware depencies might miss out on DUTs that actually don't
	// support keyboard backlight. "EC_KB_BL_EN" and "KB_BL_EN" appear to be
	// two common names for the gpio in control. Checking whether these gpios
	// exist would probably help with better sorting out the false positives.
	// The list may expand to include more gpio names.
	kbLightGpioNames := []string{"EC_KB_BL_EN", "KB_BL_EN"}
	s.Logf("Checking if the following keyboard backlight gpios exist: %s", strings.Join(kbLightGpioNames, ", "))
	if err := grepKbLightGPIO(ctx, h, kbLightGpioNames); err != nil {
		s.Log("Unexpected output when checking on gpio: ", err)
	}

	s.Log("Checking for available led paths")
	ledPaths := "/sys/class/leds"
	out, err := s.DUT().Conn().CommandContext(ctx, "ls", ledPaths).Output()
	if err != nil {
		s.Log("Could not list '/sys/class/leds': ", err)
	} else {
		var paths []string
		for _, val := range strings.Split(string(out), "\n") {
			if val == "" {
				continue
			}
			paths = append(paths, strings.TrimSpace(val))
		}
		s.Logf("Found %s", paths)
	}

	initValue, err := checkInitKBBacklight(ctx, h)
	if err != nil {
		s.Fatal("Failed to check initial keybaord backlight value: ", err)
	}
	switch initValue {
	case 0:
		s.Log("Keyboard initial backlight value is 0, attempting to increase the light to at least 40 percent before test")
		err = adjustKBBacklight(ctx, h, s.DUT(), 40, 15*time.Second, "<f7>", "increasing")
	case 100:
		s.Log("Keyboard initial backlight value is 100, attempting to decrease the light to at leaset 40 percent before test")
		err = adjustKBBacklight(ctx, h, s.DUT(), 40, 15*time.Second, "<f6>", "decreasing")
	}
	if err != nil {
		if _, ok := err.(*timeoutError); ok {
			s.Fatal("Test ended: ", err.(*timeoutError))
		} else {
			s.Fatal("Unexpected error: ", err)
		}
	}

	kbBacklightTesting := make(map[int]string, 2)
	kbBacklightTesting[0] = "<f6>"
	kbBacklightTesting[100] = "<f7>"

	for extremeValue, key := range kbBacklightTesting {
		s.Logf("-----Adjusting keyboard backlight till %d percent-----", extremeValue)
		if err := adjustKBBacklight(ctx, h, s.DUT(), extremeValue, 15*time.Second, key, ""); err != nil {
			s.Fatal("Failed to adjust keyboard backlight: ", err)
		}
	}
}

// checkInitKBBacklight presses on a key and checks the initial keyboard backlight value.
func checkInitKBBacklight(ctx context.Context, h *firmware.Helper) (int, error) {
	// Press on a key and check the initial keyboard brightness value.
	if err := h.Servo.KeypressWithDuration(ctx, servo.Enter, servo.DurPress); err != nil {
		return 0, errors.Wrap(err, "failed to press ENTER to check initial kb backlight")
	}
	// Delay by 1 second to wait for keyboard to be lit up.
	if err := testing.Sleep(ctx, time.Second); err != nil {
		return 0, errors.Wrap(err, "error in sleeping for 1 second after pressing on the ENTER key")
	}

	kbLight, err := h.Servo.GetKBBacklight(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get kb backlight")
	}

	return kbLight, nil
}

// shouldContinue continues adjustment on keyboard backlight until reaching the desired value.
func shouldContinue(kbBacklight, extremeValue int, action string) bool {
	switch action {
	case "increasing":
		return kbBacklight <= extremeValue
	case "decreasing":
		return kbBacklight >= extremeValue
	default:
		return kbBacklight != extremeValue
	}
}

// adjustKBBacklight attempts to adjust keyboard backlight within a certain duration of time.
// If a timeout is reached, possibly because no physical kb light exists, some information will
// be logged regarding pwm values, and values from files evaluated in hwdep.
func adjustKBBacklight(ctx context.Context, h *firmware.Helper, d *dut.DUT, extremeValue int, dur time.Duration, actionKey, action string) error {
	// Check the pwm value before adjusting kb light if it exists.
	initialPwm, err := checkKbLightPwm(ctx, d)
	if err != nil {
		testing.ContextLog(ctx, "Checking initial kb light pwm failed: ", err)
	}

	kbLight, err := h.Servo.GetKBBacklight(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get kb backlight")
	}

	// Log for output from running the 'backlight_tool' command. If kb light is absent,
	// this command would return: 'No backlight in /sys/class/leds matched by *:kbd_backlight'.
	// Otherwise, it will respond with a kb light percentage value. To-do: Verify its accuracy
	// first via the test script, and potentially move it to hwdep.KeyboardBacklight() later.
	hasKbLight := true
	out, err := h.DUT.Conn().CommandContext(ctx, "backlight_tool", "--keyboard", "--get_brightness").CombinedOutput()
	if err != nil {
		testing.ContextLog(ctx, "Could not obtain output from backlight_tool: ", err)
	}
	outStr := strings.TrimSpace(string(out))
	if strings.Contains(outStr, "No backlight in") {
		hasKbLight = false
	}

	// Set a specific duration on adjusting the kb light.
	endTime := time.Now().Add(dur)
	for shouldContinue(kbLight, extremeValue, action) {
		timeNow := time.Now()
		if timeNow.After(endTime) {
			// At timeout, check the final pwm value for kb light if it exists.
			finalPwm, err := checkKbLightPwm(ctx, d)
			if err != nil {
				testing.ContextLog(ctx, "Checking final kb light pwm failed: ", err)
			}
			hwdepResults := checkKBLightDependency(ctx, h)
			return &timeoutError{E: errors.Errorf(
				"timeout in adjusting kb backlight. Got kb light initial pwm val: %s, final pwm val: %s, and hwdep val: %q, backlight_tool returns kb light present: %t",
				initialPwm, finalPwm, hwdepResults, hasKbLight)}
		}
		testing.ContextLogf(ctx, "Attempting to match, current: %d, expected: %d", kbLight, extremeValue)
		if err := pressShortcut(ctx, h, actionKey); err != nil {
			return errors.Wrap(err, "failed to adjust kb backlight brightness")
		}
		kbLight, err = h.Servo.GetKBBacklight(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get kb backlight")
		}
	}
	return nil
}

// pressShortcut presses, then releases keys to adjust keyboard backlight.
func pressShortcut(ctx context.Context, h *firmware.Helper, actionKey string) error {
	// ShortCuts for decreasing keyboard backlight: Alt+F6 (Alt+BrightnessDown).
	// ShortCuts for increasing keyboard backlight: Alt+F7 (Alt+BrightnessUp).
	if err := func(ctx context.Context) error {
		keyNames := []string{"<alt_l>", actionKey}
		for _, key := range keyNames {
			row, col, err := h.Servo.GetKeyRowCol(key)
			if err != nil {
				return errors.Wrapf(err, "failed to get key column and row for %s", key)
			}
			holdKey := fmt.Sprintf("kbpress %d %d 1", col, row)
			releaseKey := fmt.Sprintf("kbpress %d %d 0", col, row)
			// Press key.
			if err := h.Servo.RunECCommand(ctx, holdKey); err != nil {
				return errors.Wrapf(err, "failed to press and hold %s", key)
			}
			// Release key.
			defer func(ctx context.Context, releaseKey, name string) error {
				if err := h.Servo.RunECCommand(ctx, releaseKey); err != nil {
					return errors.Wrapf(err, "failed to release %s", releaseKey)
				}
				return nil
			}(ctx, releaseKey, key)
		}
		return nil
	}(ctx); err != nil {
		return err
	}
	return nil
}

// grepKbLightGPIO accepts a list of gpio names, and logs their values if found.
func grepKbLightGPIO(ctx context.Context, h *firmware.Helper, gpios []string) error {
	if err := h.Servo.RunECCommand(ctx, "chan 0"); err != nil {
		return errors.Wrap(err, "failed to send 'chan 0' to EC")
	}
	for _, name := range gpios {
		var (
			reFoundGpio    = regexp.MustCompile(fmt.Sprintf(`(?i)(0|1)[^\n\r]*\s%s`, name))
			reNotFoundGpio = regexp.MustCompile(`Parameter\s+(\d+)\s+invalid`)
			checkGpio      = `(` + reFoundGpio.String() + `|` + reNotFoundGpio.String() + `)`
		)
		cmd := fmt.Sprintf("gpioget %s", name)
		out, err := h.Servo.RunECCommandGetOutput(ctx, cmd, []string{checkGpio})
		if err != nil {
			return errors.Wrapf(err, "failed to run command %v, got error", cmd)
		}
		if match := reFoundGpio.FindStringSubmatch(out[0][0]); match != nil {
			testing.ContextLogf(ctx, "Found gpio %s with value %s", name, match[1])
		} else {
			testing.ContextLogf(ctx, "Did not find gpio: %s", name)
		}
	}
	if err := h.Servo.RunECCommand(ctx, "chan 0xffffffff"); err != nil {
		return errors.Wrap(err, "failed to send 'chan 0xffffffff' to EC")
	}
	return nil
}

// checkKbLightPwm runs the host command 'ectool pwmgetduty kb' to collect pwm value.
func checkKbLightPwm(ctx context.Context, dut *dut.DUT) (string, error) {
	reFoundValue := regexp.MustCompile(`Current PWM duty:\s*\d*`)
	reValue := regexp.MustCompile(`\d+`)
	cmd := firmware.NewECTool(dut, firmware.ECToolNameMain)
	out, err := cmd.Command(ctx, "pwmgetduty", "kb").CombinedOutput()
	if err != nil {
		msg := strings.Split(strings.TrimSpace(string(out)), "\n")
		return "", errors.Errorf("running 'ectool pwmgetduty kb' on DUT failed: %v, and received: %v", err, msg)
	}
	match := reFoundValue.FindSubmatch(out)
	if len(match) == 0 {
		return "", errors.New("did not find pwm duty for kb light")
	}
	val := reValue.FindSubmatch(out)
	pwmVal := strings.TrimSpace(string(val[0]))
	return pwmVal, nil
}

// checkKBLightDependency logs the values from files evaluated by hwdep.KeyboardBacklight.
func checkKBLightDependency(ctx context.Context, h *firmware.Helper) map[string]string {
	reFoundVal := regexp.MustCompile(`\S*`)
	knownKBLightPaths := []string{
		"/run/chromeos-config/v1/keyboard/backlight",
		"/run/chromeos-config/v1/power/has-keyboard-backlight",
		"/usr/share/power_manager/has_keyboard_backlight"}

	hwdepValsMap := make(map[string]string)
	for _, path := range knownKBLightPaths {
		kbLight, err := h.Reporter.CatFile(ctx, path)
		val := reFoundVal.FindStringSubmatch(kbLight)
		if err != nil || len(val) == 0 {
			testing.ContextLogf(ctx, "Unable to read from %s", path)
			continue
		}
		hwdepValsMap[path] = string(kbLight)
	}
	return hwdepValsMap
}
