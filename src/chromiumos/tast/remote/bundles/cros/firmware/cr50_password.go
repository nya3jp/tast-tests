// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"io"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const ccdPassword string = "robot"
const waitAfterCCDSettingChange = 3 * time.Second

func init() {
	testing.AddTest(&testing.Test{
		Func:         Cr50Password,
		Desc:         "Verify that Cr50 password can be set and cleared",
		Contacts:     []string{"tj@semihalf.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_unstable"},
		Fixture:      fixture.DevMode,
		Timeout:      10 * time.Minute,
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		SoftwareDeps: []string{"gsc"},
	})
}

func Cr50Password(ctx context.Context, s *testing.State) {
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servod")
	}

	err := h.OpenCCD(ctx, true, true)
	if err != nil {
		s.Fatal("Failed to open CCD: ", err)
	}

	defer func() {
		s.Log("Cleanup after test")
		passwordIsSet := false

		ccdState, ccdPasswd, err := getCCDStatePasswd(ctx, h)
		if err != nil {
			s.Fatal("getCCDStatePasswd() failed: ", err)
		}
		if ccdPasswd != "none" {
			passwordIsSet = true
		}
		if ccdState != "Opened" {
			if passwordIsSet {
				s.Log("Open CCD with password")
				if err := verifyCr50Command(ctx, h, "ccd open "+ccdPassword, "Opened", "set", false); err != nil {
					s.Fatal("verifyCr50Command failed: ", err)
				}
			} else {
				s.Log("Open CCD")
				if err := verifyCr50Command(ctx, h, "ccd open", "Opened", "none", false); err != nil {
					s.Fatal("verifyCr50Command failed: ", err)
				}
			}
			testing.Sleep(ctx, waitAfterCCDSettingChange)
		}
		s.Log("Reset CCD")
		if _, err = h.Servo.RunCR50CommandGetOutput(ctx, "ccd reset", []string{`Resetting\s+all\s+settings`}); err != nil {
			s.Fatal("Failed resetting: ", err)
		}
	}()

	s.Log("Setting password")
	if err := verifyGsctoolCommand(ctx, h, "setPassword", "Opened", "set", false, false, false); err != nil {
		s.Fatal("verifyGsctoolCommand failed: ", err)
	}
	s.Log("Reset CCD from Cr50 console and expect that password is cleared")
	if err := verifyCr50Command(ctx, h, "ccd reset", "Opened", "none", false); err != nil {
		s.Fatal("verifyCr50Command failed: ", err)
	}

	ccdSettings := map[servo.CCDCap]servo.CCDCapState{
		"OpenNoLongPP":  "Always", // avoid clicking power button to open CCD
		"OpenNoTPMWipe": "Always", // do not reboot on ccd open
		"OpenFromUSB":   "Always", // allow opening CCD from Cr50 console
	}
	if err := h.Servo.SetCCDCapability(ctx, ccdSettings); err != nil {
		s.Fatal("Failed to set CCD capability: ", err)
	}

	s.Log("Setting password")
	if err := verifyGsctoolCommand(ctx, h, "setPassword", "Opened", "set", false, false, false); err != nil {
		s.Fatal("verifyGsctoolCommand failed: ", err)
	}
	s.Log("Setting password while password is set")
	if err := verifyGsctoolCommand(ctx, h, "setPassword", "Opened", "set", false, true, false); err != nil {
		s.Fatal("verifyGsctoolCommand failed: ", err)
	}
	s.Log("Reboot GSC and expect that password is still set afterwards")
	if err := verifyCr50Command(ctx, h, "reboot", "Locked", "set", true); err != nil {
		s.Fatal("verifyGsctoolCommand failed: ", err)
	}
	s.Log("CCD is locked after reboot, try to open with no password and expect that it remains locked")
	if err := verifyCr50Command(ctx, h, "ccd open", "Locked", "set", false); err != nil {
		s.Fatal("verifyCr50Command failed: ", err)
	}
	s.Log("Run ccd open with password from Cr50 console and expect that ccd opens")
	if err := verifyCr50Command(ctx, h, "ccd open "+ccdPassword, "Opened", "set", false); err != nil {
		s.Fatal("verifyCr50Command failed: ", err)
	}
	s.Log("Lock CCD")
	if err := verifyGsctoolCommand(ctx, h, "lock", "Locked", "set", false, false, false); err != nil {
		s.Fatal("verifyGsctoolCommand failed: ", err)
	}
	s.Log("Try to clear password while CCD is locked")
	if err := verifyGsctoolCommand(ctx, h, "clearPassword", "Locked", "set", false, true, false); err != nil {
		s.Fatal("verifyGsctoolCommand failed: ", err)
	}
	s.Log("Run ccd unlock with password from Cr50 console and expect that CCD unlocks")
	if err := verifyCr50Command(ctx, h, "ccd unlock "+ccdPassword, "Unlocked", "set", false); err != nil {
		s.Fatal("verifyCr50Command failed: ", err)
	}
	s.Log("Clear password while CCD is unlocked")
	if err := verifyGsctoolCommand(ctx, h, "clearPassword", "Unlocked", "none", false, false, false); err != nil {
		s.Fatal("verifyGsctoolCommand failed: ", err)
	}
	s.Log("Run ccd testlab open from Cr50 console and expected that CCD opens")
	if err := verifyCr50Command(ctx, h, "ccd testlab open", "Opened", "none", false); err != nil {
		s.Fatal("verifyCr50Command failed: ", err)
	}
	s.Log("Setting password")
	if err := verifyGsctoolCommand(ctx, h, "setPassword", "Opened", "set", false, false, false); err != nil {
		s.Fatal("verifyGsctoolCommand failed: ", err)
	}
	s.Log("Try to clear password using wrong password")
	if err := verifyGsctoolCommand(ctx, h, "clearPassword", "Opened", "set", false, true, true); err != nil {
		s.Fatal("verifyGsctoolCommand failed: ", err)
	}
	s.Log("Clear password")
	if err := verifyGsctoolCommand(ctx, h, "clearPassword", "Opened", "none", false, false, false); err != nil {
		s.Fatal("verifyGsctoolCommand failed: ", err)
	}
	s.Log("Unlock CCD")
	if err := verifyGsctoolCommand(ctx, h, "unlock", "Unlocked", "none", false, false, false); err != nil {
		s.Fatal("verifyGsctoolCommand failed: ", err)
	}
	s.Log("Set the password while CCD is unlocked")
	if err := verifyGsctoolCommand(ctx, h, "setPassword", "Unlocked", "set", false, false, false); err != nil {
		s.Fatal("verifyGsctoolCommand failed: ", err)
	}
	s.Log("Lock CCD")
	if err := verifyGsctoolCommand(ctx, h, "lock", "Locked", "set", false, false, false); err != nil {
		s.Fatal("verifyGsctoolCommand failed: ", err)
	}
	s.Log("Try to clear password while CCD is locked")
	if err := verifyGsctoolCommand(ctx, h, "clearPassword", "Locked", "set", false, true, false); err != nil {
		s.Fatal("verifyGsctoolCommand failed: ", err)
	}
	s.Log("Unlock CCD while the password is set")
	if err := verifyGsctoolCommand(ctx, h, "unlock", "Unlocked", "set", false, false, false); err != nil {
		s.Fatal("verifyGsctoolCommand failed: ", err)
	}
	s.Log("Clear password")
	if err := verifyGsctoolCommand(ctx, h, "clearPassword", "Unlocked", "none", false, false, false); err != nil {
		s.Fatal("verifyGsctoolCommand failed: ", err)
	}
	s.Log("Set the password while CCD is unlocked")
	if err := verifyGsctoolCommand(ctx, h, "setPassword", "Unlocked", "set", false, false, false); err != nil {
		s.Fatal("verifyGsctoolCommand failed: ", err)
	}
	s.Log("Run ccd testlab open from Cr50 console and expect that CCD opens")
	if err := verifyCr50Command(ctx, h, "ccd testlab open", "Opened", "set", false); err != nil {
		s.Fatal("verifyCr50Command failed: ", err)
	}
	s.Log("Clear password")
	if err := verifyGsctoolCommand(ctx, h, "clearPassword", "Opened", "none", false, false, false); err != nil {
		s.Fatal("verifyGsctoolCommand failed: ", err)
	}

	s.Log("Set OpenFromUSB=IfOpened")
	ccdSettings = map[servo.CCDCap]servo.CCDCapState{
		"OpenFromUSB": "IfOpened", // when password is not set, opening CCD should be possible only with gsctool
	}
	if err := h.Servo.SetCCDCapability(ctx, ccdSettings); err != nil {
		s.Fatal("Failed to set CCD capability: ", err)
	}
	s.Log("Setting password")
	if err := verifyGsctoolCommand(ctx, h, "setPassword", "Opened", "set", false, false, false); err != nil {
		s.Fatal("verifyGsctoolCommand failed: ", err)
	}
	s.Log("Lock CCD")
	if err := verifyGsctoolCommand(ctx, h, "lock", "Locked", "set", false, false, false); err != nil {
		s.Fatal("verifyGsctoolCommand failed: ", err)
	}
	s.Log("Run ccd open with password from Cr50 console and expect that CCD opens")
	if err := verifyCr50Command(ctx, h, "ccd open "+ccdPassword, "Opened", "set", false); err != nil {
		s.Fatal("verifyCr50Command failed: ", err)
	}
	s.Log("Clear password")
	if err := verifyGsctoolCommand(ctx, h, "clearPassword", "Opened", "none", false, false, false); err != nil {
		s.Fatal("verifyGsctoolCommand failed: ", err)
	}
	s.Log("Lock CCD")
	if err := verifyGsctoolCommand(ctx, h, "lock", "Locked", "none", false, false, false); err != nil {
		s.Fatal("verifyGsctoolCommand failed: ", err)
	}
	s.Log("Try to open CCD from Cr50 console and expect that it remains locked")
	if err := verifyCr50Command(ctx, h, "ccd open", "Locked", "none", false); err != nil {
		s.Fatal("verifyCr50Command failed: ", err)
	}
	s.Log("Open CCD from developer console")
	if err := verifyGsctoolCommand(ctx, h, "open", "Opened", "none", false, false, false); err != nil {
		s.Fatal("verifyGsctoolCommand failed: ", err)
	}
}

func verifyCr50Command(ctx context.Context, h *firmware.Helper, cmd, expectCCDState, expectCCDPasswdState string, expectReboot bool) error {
	bootID, err := h.Reporter.BootID(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get boot id")
	}
	err = h.Servo.RunCR50Command(ctx, cmd)
	if err != nil {
		return errors.Wrapf(err, "failed to execute %q", cmd)
	}
	if expectReboot {
		if err := waitForDUTReboot(ctx, h, bootID); err != nil {
			return errors.Wrap(err, "reboot failed")
		}
	} else {
		testing.Sleep(ctx, waitAfterCCDSettingChange)
	}
	if err = checkExpectedCCDState(ctx, h, expectCCDState, expectCCDPasswdState); err != nil {
		return errors.Wrap(err, "checkExpectedCCDState() failed")
	}
	return nil
}

func verifyGsctoolCommand(ctx context.Context, h *firmware.Helper, behavior, expectCCDState, expectCCDPasswdState string, expectReboot, expectFail, useWrongPassword bool) error {
	params := map[string]string{
		"open":          "-o",
		"lock":          "-k",
		"unlock":        "-U",
		"setPassword":   "-P",
		"clearPassword": "-P",
	}
	passwordIsSet := false
	_, ccdPasswdState, err := getCCDStatePasswd(ctx, h)
	if err != nil {
		return errors.Wrap(err, "getCCDStatePasswd() failed")
	}
	if ccdPasswdState != "none" {
		passwordIsSet = true
	}

	bootID, err := h.Reporter.BootID(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get boot id")
	}

	cmd := h.DUT.Conn().CommandContext(ctx, "gsctool", "-a", params[behavior])
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return errors.Wrap(err, "StdinPipe() failed")
	}
	defer func() {
		cmd.Wait()
	}()
	testing.ContextLog(ctx, "Starting gsctool")
	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "Start() failed")
	}

	ccdPasswd := ccdPassword
	if useWrongPassword {
		ccdPasswd = ccdPasswd + "@"
	}
	if behavior == "clearPassword" {
		ccdPasswd = "clear:" + ccdPasswd
	}

	if behavior == "setPassword" || behavior == "clearPassword" || passwordIsSet {
		testing.ContextLog(ctx, "Entering password")
		// Enter password twice
		if _, err := io.WriteString(stdin, ccdPasswd+"\n"+ccdPasswd+"\n"); err != nil {
			return errors.Wrap(err, "WriteString() failed")
		}
	}
	if err := cmd.Wait(); err != nil {
		if !expectFail {
			return errors.Wrap(err, "gsctool failed")
		}
	}
	if expectReboot {
		if err := waitForDUTReboot(ctx, h, bootID); err != nil {
			return errors.Wrap(err, "reboot failed")
		}
	} else {
		testing.Sleep(ctx, waitAfterCCDSettingChange)
	}

	if err = checkExpectedCCDState(ctx, h, expectCCDState, expectCCDPasswdState); err != nil {
		return errors.Wrap(err, "checkExpectedCCDState() failed")
	}
	return nil
}

func waitForDUTReboot(ctx context.Context, h *firmware.Helper, bootID string) error {
	testing.ContextLog(ctx, "Waiting for connection to DUT")
	reconnectTimeout := 3 * time.Minute
	connectCtx, cancel := context.WithTimeout(ctx, reconnectTimeout)
	defer cancel()
	if err := h.WaitConnect(connectCtx); err != nil {
		return errors.Wrap(err, "failed to connect to DUT")
	}
	newBootID, err := h.Reporter.BootID(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get new boot id")
	}
	if newBootID == bootID {
		return errors.Wrap(err, "unexpectedly got same boot id over reboot")
	}
	return nil
}

func getCCDStatePasswd(ctx context.Context, h *firmware.Helper) (string, string, error) {
	out, err := h.Servo.RunCR50CommandGetOutput(ctx, "ccd", []string{`State:\s*(\S+)\s*\n\s*Password:\s*(\S+)`})
	if err != nil {
		return "", "", errors.Wrap(err, "function RunCR50CommandGetOutput() returned an error")
	}
	return out[0][1], out[0][2], nil
}

func checkExpectedCCDState(ctx context.Context, h *firmware.Helper, expectCCDState, expectCCDPasswdState string) error {
	ccdState, ccdPasswdState, err := getCCDStatePasswd(ctx, h)
	if err != nil {
		return errors.Wrap(err, "getCCDStatePasswd() failed")
	}
	if ccdState != expectCCDState {
		err = errors.Errorf("expected CCD state: %q, got %q", expectCCDState, ccdState)
		if ccdPasswdState != expectCCDPasswdState {
			err = errors.Errorf("%s; expected CCD password state: %q, got %q", err, expectCCDPasswdState, ccdPasswdState)
		}
	} else if ccdPasswdState != expectCCDPasswdState {
		err = errors.Errorf("expected CCD password state: %q, got %q", expectCCDPasswdState, ccdPasswdState)
	}
	if err != nil {
		return errors.Wrap(err, "unexpected CCD state")
	}
	testing.ContextLog(ctx, "CCD state as expected")
	return nil
}
