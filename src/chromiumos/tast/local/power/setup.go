// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/power/ectool"
	"chromiumos/tast/local/testexec"
)

// Setup tracks all the things we need to do to put the DUT back to how it
// was configured before this power test.
type Setup struct {
	ctx       context.Context
	callbacks []func()
	err       error
}

// NewSetup creates a new test setup and cleanup struct.
func NewSetup(ctx context.Context) *Setup {
	return &Setup{ctx: ctx, callbacks: []func(){}, err: nil}
}

// append adds a cleanup task to the setup.
func (s *Setup) append(callback func()) {
	if s.err != nil {
		callback()
		return
	}
	s.callbacks = append(s.callbacks, callback)
}

// fail checks a result needed for setup. If there is an error, then
// the setup is marked as failed, and all previous cleanup tasks are run.
func (s *Setup) fail(err error) {
	if s.err != nil {
		return
	}
	s.Cleanup()
	s.err = err
}

// Cleanup restores a DUT to is pre-test configuration.
func (s *Setup) Cleanup() {
	for _, callback := range s.callbacks {
		callback()
	}
	s.callbacks = []func(){}
}

// Error checks to see if the setup was successful.
func (s *Setup) Error() error {
	return s.err
}

// serviceStatusRe is used to parse the result of a service status command.
var serviceStatusRe = regexp.MustCompile("^.* (start/running, process \\d+)|(stop/waiting)\n$")

const (
	serviceMissing = iota
	serviceRunning
	serviceStopped
)

// serviceStatus returns the status of a service.
func serviceStatus(ctx context.Context, serviceName string) (int, error) {
	output, err := testexec.CommandContext(ctx, "status", serviceName).Output()
	if err != nil {
		return serviceMissing, nil
	}
	match := serviceStatusRe.FindSubmatch(output)
	if match == nil {
		return serviceMissing, errors.Wrapf(err, "unable to parse status %q of service %q", output, serviceName)
	}
	if match[1] != nil {
		return serviceRunning, nil
	} else if match[2] != nil {
		return serviceStopped, nil
	}
	return serviceMissing, nil
}

// StopService stops a service if it is running, and updates cleanup callbacks
// to restart the service.
func (s *Setup) StopService(serviceName string) {
	if s.Error() != nil {
		return
	}
	prevStatus, err := serviceStatus(s.ctx, serviceName)
	if err != nil {
		s.fail(err)
		return
	}
	if prevStatus != serviceRunning {
		// Service is not running, so we don't need to do anything.
		return
	}
	if err := testexec.CommandContext(s.ctx, "stop", serviceName).Run(testexec.DumpLogOnError); err != nil {
		s.fail(errors.Wrap(err, "unable to stop service"))
		return
	}
	s.append(func() {
		testexec.CommandContext(s.ctx, "start", serviceName).Run(testexec.DumpLogOnError)
	})
}

// getBacklightBrightness returns the current backlight brightness in percent.
func getBacklightBrightness(ctx context.Context) (uint, error) {
	output, err := testexec.CommandContext(ctx, "backlight_tool", "--get_brightness").Output(testexec.DumpLogOnError)
	if err != nil {
		return 0, errors.Wrap(err, "unable to get current backlight brightness")
	}
	brightness, err := strconv.ParseUint(strings.TrimSpace(string(output)), 10, 64)
	if err != nil {
		return 0, errors.Wrapf(err, "unable to parse current backlight brightness from %q", output)
	}
	return uint(brightness), nil
}

// getDefaultBacklightBrightness returns the backlight brightness at a given
// lux level.
func getDefaultBacklightBrightness(ctx context.Context, lux uint) (uint, error) {
	luxArg := "--lux=" + strconv.FormatUint(uint64(lux), 10)
	output, err := testexec.CommandContext(ctx, "backlight_tool", "--get_initial_brightness", luxArg).Output(testexec.DumpLogOnError)
	if err != nil {
		return 0, errors.Wrap(err, "unable to get default backlight brightness")
	}
	brightness, err := strconv.ParseUint(strings.TrimSpace(string(output)), 10, 64)
	if err != nil {
		return 0, errors.Wrap(err, "unable to parse default backlight brightness")
	}
	return uint(brightness), nil
}

// setBacklightBrightness sets the backlight brightness.
func setBacklightBrightness(ctx context.Context, brightness uint) error {
	brightnessArg := "--set_brightness=" + strconv.FormatUint(uint64(brightness), 10)
	if err := testexec.CommandContext(ctx, "backlight_tool", brightnessArg).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "unable to set backlight brightness")
	}
	return nil
}

// SetBacklightBrightness sets the backlight to have a given lux level, and
// updates cleanup callbacks to restore the backlight to it's original
// brightness.
func (s *Setup) SetBacklightBrightness(lux uint) {
	if s.Error() != nil {
		return
	}
	prevBrightness, err := getBacklightBrightness(s.ctx)
	if err != nil {
		s.fail(err)
		return
	}
	targetBrightness, err := getDefaultBacklightBrightness(s.ctx, lux)
	if err != nil {
		s.fail(err)
		return
	}
	if err := setBacklightBrightness(s.ctx, targetBrightness); err != nil {
		s.fail(err)
		return
	}
	s.append(func() {
		setBacklightBrightness(s.ctx, prevBrightness)
	})
}

// getKeyboardBrightness gets the keyboard brightness.
func getKeyboardBrightness(ctx context.Context) (uint, error) {
	output, err := testexec.CommandContext(ctx, "backlight_tool", "--keyboard", "--get_brightness").Output(testexec.DumpLogOnError)
	if err != nil {
		return 0, errors.Wrap(err, "unable to get current keyboard brightness")
	}
	brightness, err := strconv.ParseUint(strings.TrimSpace(string(output)), 10, 64)
	if err != nil {
		return 0, errors.Wrapf(err, "unable to parse current keyboard brightness from %q", output)
	}
	return uint(brightness), nil
}

// setKeyboardBrightness sets the keyboard brightness.
func setKeyboardBrightness(ctx context.Context, brightness uint) error {
	brightnessArg := "--set_brightness=" + strconv.FormatUint(uint64(brightness), 10)
	if err := testexec.CommandContext(ctx, "backlight_tool", "--keyboard", brightnessArg).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "unable to set keyboard brightness")
	}
	return nil
}

// SetKeyboardBrightness sets the keyboard brightness and updates cleanup
// callbacks to restore the keyboard backlight to it's original brightness.
func (s *Setup) SetKeyboardBrightness(brightness uint) {
	if s.Error() != nil {
		return
	}
	prevBrightness, err := getKeyboardBrightness(s.ctx)
	if err != nil {
		s.fail(err)
		return
	}
	if err := setKeyboardBrightness(s.ctx, brightness); err != nil {
		s.fail(err)
		return
	}
	s.append(func() {
		setKeyboardBrightness(s.ctx, prevBrightness)
	})
}

// crasTestClientRe parses the output of cras_test_client.
var crasTestClientRe = regexp.MustCompile("^System Volume \\(0-100\\): (\\d+) (\\(Muted\\))?\nCapture Gain \\(-?\\d+\\.\\d+ - -?\\d+\\.\\d+\\): (-?\\d+\\.\\d+)dB \n")

// readAudioSettings reads the volume, recorder gain in decibels, and system
// mute state.
func readAudioSettings(ctx context.Context) (uint, float64, bool, error) {
	output, err := testexec.CommandContext(ctx, "cras_test_client").Output(testexec.DumpLogOnError)
	if err != nil {
		return 0.0, 0.0, false, errors.Wrap(err, "unable to call cras_test_client")
	}
	match := crasTestClientRe.FindSubmatch(output)
	if match == nil {
		return 0.0, 0.0, false, errors.Wrapf(err, "unable to parse audio settings from output %q", output)
	}
	volume, err := strconv.ParseUint(string(match[1]), 10, 64)
	if err != nil {
		return 0.0, 0.0, false, errors.Wrapf(err, "unable to parse volume from %q", match[1])
	}
	muted := match[2] != nil
	gain, err := strconv.ParseFloat(string(match[3]), 64)
	if err != nil {
		return 0.0, 0.0, false, errors.Wrapf(err, "unable to parse gain from %q", match[3])
	}
	return uint(volume), gain, muted, nil
}

// setAudioVolume sets the system audio volume level.
func setAudioVolume(ctx context.Context, volume uint) error {
	volumeArg := strconv.FormatUint(uint64(volume), 10)
	if err := testexec.CommandContext(ctx, "cras_test_client", "--volume", volumeArg).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "unable set audio volume")
	}
	return nil
}

// setAudioMuted enables or disables the system mute.
func setAudioMuted(ctx context.Context, muted bool) error {
	mutedArg := "0"
	if muted {
		mutedArg = "1"
	}
	if err := testexec.CommandContext(ctx, "cras_test_client", "--mute", mutedArg).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "unable set audio mute")
	}
	return nil
}

// setAudioGain sets the audio recorder gain in decibels.
func setAudioGain(ctx context.Context, gain float64) error {
	// The --capture_gain argument takes a value in millibel
	const milliInDeci = 100
	gainArg := strconv.FormatInt(int64(gain*milliInDeci), 10)
	if err := testexec.CommandContext(ctx, "cras_test_client", "--capture_gain", gainArg).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "unable set capture gain")
	}
	return nil
}

// MuteAudio sets the volume to 0, enables the system mute, sets the audio
// recorder gain to 0, and updates cleanup callbacks to restore previous
// values.
func (s *Setup) MuteAudio() {
	if s.Error() != nil {
		return
	}
	prevVolume, prevGain, prevMuted, err := readAudioSettings(s.ctx)
	if err != nil {
		s.fail(err)
		return
	}
	if err := setAudioVolume(s.ctx, 0); err != nil {
		s.fail(err)
		return
	}
	if err := setAudioMuted(s.ctx, true); err != nil {
		s.fail(err)
		return
	}
	if err := setAudioGain(s.ctx, 0.0); err != nil {
		s.fail(err)
		return
	}
	s.append(func() {
		setAudioVolume(s.ctx, prevVolume)
		setAudioMuted(s.ctx, prevMuted)
		setAudioGain(s.ctx, prevGain)
	})
}

// ifconfigRe parses one adapter from the output of ifconfig.
var ifconfigRe = regexp.MustCompile("([^:]+): .*\n(?: +.*\n)*\n")

// parseIfconfigOutput gets a list of adapters from ifconfig output.
func parseIfconfigOutput(output []byte) ([]string, error) {
	var interfaces []string
	match := ifconfigRe.FindAllSubmatch(output, -1)
	if match == nil {
		return interfaces, errors.Errorf("unable to parse interface list from %q", output)
	}
	for _, submatch := range match {
		interfaces = append(interfaces, string(submatch[1]))
	}
	return interfaces, nil
}

// DisableNetworkInterfaces disables all network interfaces that match the
// passed pattern, and updates cleanup callbacks to reenable them.
func (s *Setup) DisableNetworkInterfaces(pattern *regexp.Regexp) {
	if s.Error() != nil {
		return
	}

	// Build a list of all network interfaces.
	output, err := testexec.CommandContext(s.ctx, "ifconfig", "-a").Output(testexec.DumpLogOnError)
	if err != nil {
		s.fail(errors.Wrap(err, "unable to get interface list"))
		return
	}
	allInterfaces, err := parseIfconfigOutput(output)
	if err != nil {
		s.fail(err)
		return
	}

	// Build a map of network interfaces that are up.
	output, err = testexec.CommandContext(s.ctx, "ifconfig").Output(testexec.DumpLogOnError)
	if err != nil {
		s.fail(errors.Wrap(err, "unable to get interface list"))
		return
	}
	upInterfaces, err := parseIfconfigOutput(output)
	if err != nil {
		s.fail(err)
		return
	}
	upMap := make(map[string]bool)
	for _, iface := range upInterfaces {
		upMap[iface] = true
	}

	// Disable any matching interface that is up.
	for _, iface := range allInterfaces {
		if !pattern.MatchString(iface) {
			continue
		}
		if _, up := upMap[iface]; !up {
			continue
		}
		if err := testexec.CommandContext(s.ctx, "ifconfig", iface, "down").Run(testexec.DumpLogOnError); err != nil {
			s.fail(errors.Wrapf(err, "unable to disable network interface %q", iface))
			return
		}
		s.append(func() {
			testexec.CommandContext(s.ctx, "ifconfig", iface, "up").Run(testexec.DumpLogOnError)
		})
	}
}

// SetBatteryDischarge forces the battery to discharge, even when on AC, and
// updates cleanup callbacks to enable charging. An error is returned if the
// battery is within the passed margin of system shutdown.
func (s *Setup) SetBatteryDischarge(lowBatteryMargin float64) {
	low, err := ectool.LowBatteryShutdownPercent(s.ctx)
	if err != nil {
		s.fail(err)
		return
	}
	b, err := ectool.NewBatteryState(s.ctx)
	if err != nil {
		s.fail(err)
		return
	}
	if b.Discharging() {
		return
	}
	if (low + lowBatteryMargin) >= b.ChargePercent() {
		s.fail(errors.Errorf("battery percent %.2f is too low to start discharging", b.ChargePercent()))
		return
	}
	if err := testexec.CommandContext(s.ctx, "ectool", "chargecontrol", "discharge").Run(testexec.DumpLogOnError); err != nil {
		s.fail(errors.Wrap(err, "unable to set battery to discharge"))
		return
	}
	s.append(func() {
		// NB: we don't restore charge state to what it was before because it's
		// probably a bad idea to let the battery run out.
		testexec.CommandContext(s.ctx, "ectool", "chargecontrol", "normal").Run(testexec.DumpLogOnError)
	})
}

// NewDefaultSetup prepares a DUT to have a power test run by consistently
// configuring power draining components and disabling sources of variance.
func NewDefaultSetup(ctx context.Context) *Setup {
	s := NewSetup(ctx)
	s.StopService("powerd")
	s.StopService("update-engine")
	s.StopService("vnc")
	s.StopService("dptf")
	s.SetBacklightBrightness(150)
	s.SetKeyboardBrightness(24)
	s.MuteAudio()
	var wifiInterfaceRe = regexp.MustCompile(".*wlan\\d+")
	s.DisableNetworkInterfaces(wifiInterfaceRe)
	s.SetBatteryDischarge(2.0)

	// TODO: bluetooth
	// TODO: SetLightbarBrightness
	// TODO: nightlight off
	return s
}
