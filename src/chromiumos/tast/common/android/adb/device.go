// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package adb

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// Device holds the resources required to communicate with a specific ADB device.
type Device struct {
	// TransportID is used to distinguish the specific device.
	TransportID string

	// Serial is used as a backup to distinguish the specific device if TransportID is empty.
	Serial string

	// These are properties of the device returned by `adb devices -l` that may be blank.
	Device  string
	Model   string
	Product string
}

// Devices returns a list of currently known ADB devices.
func Devices(ctx context.Context) ([]*Device, error) {
	output, err := Command(ctx, "devices", "-l").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to query ADB devices")
	}
	var devices []*Device
	for _, line := range strings.Split(string(output), "\n") {
		device, err := parseDevice(line)
		if err != nil {
			// Log unexpected errors but continue processing other devices.
			if !errors.Is(err, errSkippedLine) {
				testing.ContextLogf(ctx, "Failed to parse line %q, got error: %v", line, err)
			}
			continue
		}
		devices = append(devices, device)
	}
	return devices, nil
}

// Potential errors that can be returned from calling parseDevice.
var errSkippedLine = errors.New("skipped line")
var errUnexpectedLine = errors.New("'adb devices -l' ran into unexpected line")
var errUnexpectedDeviceState = errors.New("'adb devices -l' returned unexpected device state")

// parseDevice parses a line from the output of the `adb devices -l` command.
// It returns a Device on success and an error on failure.
func parseDevice(line string) (*Device, error) {
	// Ignore empty lines, comments, and header.
	if strings.TrimSpace(line) == "" || line[0] == '*' || line == "List of devices attached" {
		return nil, errSkippedLine
	}
	fields := strings.Fields(line)
	// Log info if the line does not at least contain serial and state.
	if len(fields) < 2 {
		return nil, errUnexpectedLine
	}
	// Ensure that state is valid or ignore the line.
	if _, err := parseState(fields[1]); err != nil {
		return nil, errUnexpectedDeviceState
	}
	device := &Device{
		Serial: fields[0],
	}
	for _, field := range fields[2:] {
		if strings.HasPrefix(field, "device:") {
			device.Device = strings.TrimPrefix(field, "device:")
		} else if strings.HasPrefix(field, "model:") {
			device.Model = strings.TrimPrefix(field, "model:")
		} else if strings.HasPrefix(field, "product:") {
			device.Product = strings.TrimPrefix(field, "product:")
		} else if strings.HasPrefix(field, "transport_id:") {
			device.TransportID = strings.TrimPrefix(field, "transport_id:")
		}
	}
	return device, nil
}

// WaitForDevice waits for an ADB device with the set properties.
func WaitForDevice(ctx context.Context, predicate func(device *Device) bool, timeout time.Duration) (*Device, error) {
	var device *Device
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		devices, err := Devices(ctx)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get the devices"))
		}
		for _, d := range devices {
			if predicate(d) {
				device = d
				return nil
			}
		}
		return errors.New("no device satisfies the condition")
	}, &testing.PollOptions{Interval: time.Second, Timeout: timeout}); err != nil {
		return nil, err
	}
	return device, nil
}

// Connect keeps trying to connect to an ADB at the specified address.
// Returns the device if the connection succeeds.
func Connect(ctx context.Context, addr string, timeout time.Duration) (*Device, error) {
	var device *Device
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := Command(ctx, "connect", addr).Run(testexec.DumpLogOnError); err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to run adb connect"))
		}
		devices, err := Devices(ctx)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get the devices"))
		}
		for _, d := range devices {
			if d.Serial == addr {
				device = d
				return nil
			}
		}
		return errors.New("device not connected yet")
	}, &testing.PollOptions{Interval: time.Second, Timeout: timeout}); err != nil {
		return nil, err
	}
	return device, nil
}

// Command creates an ADB command on the specified device.
func (d *Device) Command(ctx context.Context, args ...string) *testexec.Cmd {
	// ADB has an issue where if there are multiple devices connected and you try to forward a port with TransportID,
	// it will fail with "error: unknown host service". Until this is fixed, disable TransportID.
	// if d.TransportID != "" {
	//	return Command(ctx, append([]string{"-t", d.TransportID}, args...)...)
	// }
	// Use Serial as a backup if TransportID is empty.
	return Command(ctx, append([]string{"-s", d.Serial}, args...)...)
}

// State describes the state of an ADB device.
type State string

// Possible ADB device states as listed at https://developer.android.com/studio/command-line/adb#devicestatus
const (
	StateOffline  State = "offline"
	StateDevice   State = "device"
	StateNoDevice State = "no device"
	// StateUnknown is only used when an error is returned and the state is unknown.
	StateUnknown State = "unknown"
)

// parseState takes a string, trims the spaces and attempts to map it to a State.
// On failure, an error is returned with StateUnknown.
func parseState(state string) (State, error) {
	trimedState := strings.TrimSpace(state)
	if trimedState == string(StateOffline) {
		return StateOffline, nil
	} else if trimedState == string(StateDevice) {
		return StateDevice, nil
	} else if trimedState == string(StateNoDevice) {
		return StateNoDevice, nil
	}
	return StateUnknown, errors.Errorf("failed to parse state from %q", state)
}

// State gets the state of an ADB device.
func (d *Device) State(ctx context.Context) (State, error) {
	bstdout, bstderr, err := d.Command(ctx, "get-state").SeparatedOutput()
	if err != nil {
		stderr := string(bstderr)
		if strings.Contains(stderr, "device offline") {
			return StateOffline, nil
		}
		return StateUnknown, errors.Wrapf(err, "failed to get device state: %q", stderr)
	}
	return parseState(string(bstdout))
}

// WaitForState waits for the device state to be equal to the state passed in.
func (d *Device) WaitForState(ctx context.Context, want State, timeout time.Duration) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		got, err := d.State(ctx)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get the device state"))
		}
		if got != want {
			return errors.Errorf("incorrect device state(got: %v, want: %v)", got, want)
		}
		return nil
	}, &testing.PollOptions{Interval: time.Second, Timeout: timeout})
}

// IsConnected checks if the device is connected.
func (d *Device) IsConnected(ctx context.Context) error {
	if state, err := d.State(ctx); err != nil {
		return errors.Wrap(err, "failed to get the ADB device state")
	} else if state != StateDevice {
		return errors.New("ADB device not connected")
	}
	return nil
}

// InstallOption defines possible options to pass to "adb install".
type InstallOption string

// ADB install options listed in "adb help".
const (
	InstallOptionLockApp               InstallOption = "-l"
	InstallOptionReplaceApp            InstallOption = "-r"
	InstallOptionAllowTestPackage      InstallOption = "-t"
	InstallOptionSDCard                InstallOption = "-s"
	InstallOptionAllowVersionDowngrade InstallOption = "-d"
	InstallOptionGrantPermissions      InstallOption = "-g"
	InstallOptionEphemeralInstall      InstallOption = "--instant"
	InstallOptionFromPlayStore         InstallOption = "-i com.android.vending"
)

var showAPKPathWarningOnce sync.Once

// install install an APK or an split APK to the Android system.
// By default, it uses InstallOptionReplaceApp and InstallOptionAllowVersionDowngrade.
func (d *Device) install(ctx context.Context, adbCommand string, apks []string, installOptions ...InstallOption) error {
	for _, apk := range apks {
		if strings.HasPrefix(apk, apkPathPrefix) {
			showAPKPathWarningOnce.Do(func() {
				testing.ContextLog(ctx, "WARNING: When files under tast-tests/android are modified, APKs on the DUT should be pushed manually. See tast-tests/android/README.md")
			})
			break

		}
	}

	if err := d.ShellCommand(ctx, "settings", "put", "global", "verifier_verify_adb_installs", "0").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed disabling verifier_verify_adb_installs")
	}

	installOptions = append(installOptions, InstallOptionReplaceApp)
	installOptions = append(installOptions, InstallOptionAllowVersionDowngrade)
	commandArgs := []string{adbCommand}
	for _, installOption := range installOptions {
		for _, option := range strings.Split(string(installOption), " ") {
			commandArgs = append(commandArgs, option)
		}
	}
	commandArgs = append(commandArgs, apks...)
	out, err := d.Command(ctx, commandArgs...).Output(testexec.DumpLogOnError)
	if err != nil {
		return err
	}

	// "Success" is the only possible positive result. See runInstall() here:
	// https://android.googlesource.com/platform/frameworks/base/+/bdd94d9979e28c39539e25fbb98621df3cbe86f2/services/core/java/com/android/server/pm/PackageManagerShellCommand.java#901
	matched, err := regexp.Match("^Success", out)
	if err != nil {
		return err
	}
	if !matched {
		return errors.Errorf("failed to install %v %q", apks, string(out))
	}
	return nil
}

// Install installs an APK file to the Android system.
// By default, it uses InstallOptionReplaceApp and InstallOptionAllowVersionDowngrade.
func (d *Device) Install(ctx context.Context, path string, installOptions ...InstallOption) error {
	return d.install(ctx, "install", []string{path}, installOptions...)
}

// InstallMultiple installs a split APK to the Android system.
// By default, it uses InstallOptionReplaceApp and InstallOptionAllowVersionDowngrade.
func (d *Device) InstallMultiple(ctx context.Context, apks []string, installOptions ...InstallOption) error {
	return d.install(ctx, "install-multiple", apks, installOptions...)
}

// InstalledPackages returns a set of currently-installed packages, e.g. "android".
// This operation is slow (700+ ms), so unnecessary calls should be avoided.
func (d *Device) InstalledPackages(ctx context.Context) (map[string]struct{}, error) {
	ctx, st := timing.Start(ctx, "installed_packages")
	defer st.End()

	out, err := d.ShellCommand(ctx, "pm", "list", "packages").CombinedOutput(testexec.DumpLogOnError)
	if err != nil {
		if strings.Contains(string(out), "Can't find service: package") {
			return nil, errors.Wrap(err, "package service not running")
		}
		return nil, errors.Wrap(err, "listing packages failed")
	}

	pkgs := make(map[string]struct{})
	for _, pkg := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		// |pm list packages| prepends "package:" to installed packages. Not needed.
		n := strings.TrimPrefix(pkg, "package:")
		pkgs[n] = struct{}{}
	}
	return pkgs, nil
}

// PackageInstalled returns true if the given package has been installed.
func (d *Device) PackageInstalled(ctx context.Context, pkg string) (bool, error) {
	ctx, st := timing.Start(ctx, "verify_package_installed")
	defer st.End()

	// Send "pm list packages <filter>" command with the given package name as filter.
	out, err := d.ShellCommand(ctx, "pm", "list", "packages", pkg).Output(testexec.DumpLogOnError)
	if err != nil {
		return false, errors.Wrap(err, "listing packages failed")
	}

	// Check the output contains the exact package name.
	// Note that "pm list packages <filter>" returns all packages containing the given filter string.
	// It also prepends "package:" to installed package names.
	return strings.Contains(string(out), fmt.Sprintf("package:%s\n", pkg)), nil
}

// Uninstall uninstalls a package from the Android system.
func (d *Device) Uninstall(ctx context.Context, pkg string) error {
	out, err := d.Command(ctx, "uninstall", pkg).Output(testexec.DumpLogOnError)
	if err != nil {
		return err
	}

	// "Success" is the only possible positive result. See runUninstall() here:
	// https://android.googlesource.com/platform/frameworks/base/+/bdd94d9979e28c39539e25fbb98621df3cbe86f2/services/core/java/com/android/server/pm/PackageManagerShellCommand.java#1428
	matched, err := regexp.Match("^Success", out)
	if err != nil {
		return err
	}
	if !matched {
		return errors.Errorf("failed to uninstall %v %q", pkg, string(out))
	}
	return nil
}

// ForwardTCP forwards the ADB device local port specified to a host port and returns that host port.
func (d *Device) ForwardTCP(ctx context.Context, androidPort int) (int, error) {
	out, err := d.Command(ctx, "forward", "tcp:0", fmt.Sprintf("tcp:%d", androidPort)).Output(testexec.DumpLogOnError)
	if err != nil {
		return -1, err
	}
	return strconv.Atoi(strings.TrimSpace(string(out)))
}

// RemoveForwardTCP removes the forwarding from an ADB device local port to the specified host port.
func (d *Device) RemoveForwardTCP(ctx context.Context, hostPort int) error {
	return d.Command(ctx, "forward", "--remove", fmt.Sprintf("tcp:%d", hostPort)).Run(testexec.DumpLogOnError)
}

// ReverseTCP forwards the host port to an ADB device local port and returns that ADB device port.
func (d *Device) ReverseTCP(ctx context.Context, hostPort int) (int, error) {
	out, err := d.Command(ctx, "reverse", "tcp:0", fmt.Sprintf("tcp:%d", hostPort)).Output(testexec.DumpLogOnError)
	if err != nil {
		return -1, err
	}
	return strconv.Atoi(strings.TrimSpace(string(out)))
}

// RemoveReverseTCP removes the forwarding from a host port to the specified ADB device local port.
func (d *Device) RemoveReverseTCP(ctx context.Context, androidPort int) error {
	return d.Command(ctx, "reverse", "--remove", fmt.Sprintf("tcp:%d", androidPort)).Run(testexec.DumpLogOnError)
}

// PressKeyCode sends a key event with the specified key code.
func (d *Device) PressKeyCode(ctx context.Context, keycode string) error {
	return d.ShellCommand(ctx, "input", "keyevent", keycode).Run(testexec.DumpLogOnError)
}

// SDKVersion returns the Android SDK version.
func (d *Device) SDKVersion(ctx context.Context) (int, error) {
	sdkVersion, err := d.ShellCommand(ctx, "getprop", "ro.build.version.sdk").Output(testexec.DumpLogOnError)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSuffix(string(sdkVersion), "\n"))
}

// AndroidVersion returns the Android version.
func (d *Device) AndroidVersion(ctx context.Context) (int, error) {
	androidVersion, err := d.ShellCommand(ctx, "getprop", "ro.build.version.release").Output(testexec.DumpLogOnError)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSuffix(string(androidVersion), "\n"))
}

// GMSCoreVersion returns the GMS Core version.
func (d *Device) GMSCoreVersion(ctx context.Context) (int, error) {
	versionInfo, err := d.ShellCommand(ctx, "sh", "-c", "dumpsys package com.google.android.gms | grep versionCode").Output(testexec.DumpLogOnError)
	if err != nil {
		return 0, err
	}

	const versionCodePattern = "versionCode=([0-9]+)"
	r, err := regexp.Compile(versionCodePattern)
	if err != nil {
		return 0, errors.Wrap(err, "failed to compile versionCode pattern")
	}
	versionCodeMatch := r.FindStringSubmatch(string(versionInfo))
	if len(versionCodeMatch) == 0 {
		return 0, errors.New("GMS Core version number not found in command output")
	}
	version, err := strconv.Atoi(versionCodeMatch[1])
	if err != nil {
		return 0, errors.Wrapf(err, "failed to convert GMS Core version %v to int", versionCodeMatch[0])
	}
	return version, nil
}

// GoogleAccount returns the first found Google account signed in to the Android device.
func (d *Device) GoogleAccount(ctx context.Context) (string, error) {
	accountInfo, err := d.ShellCommand(ctx, "sh", "-c", "dumpsys account all | grep Account").Output(testexec.DumpLogOnError)
	if err != nil {
		return "", err
	}

	const accountPattern = "{name=([^@^}]+@[^,^}]+), type=com.google}"
	r, err := regexp.Compile(accountPattern)
	if err != nil {
		return "", errors.Wrap(err, "failed to compile Google account pattern")
	}
	accountMatch := r.FindStringSubmatch(string(accountInfo))
	if len(accountMatch) == 0 {
		return "", errors.New("Google account not found on the device")
	}
	return accountMatch[1], nil
}

// Root restarts adbd with root permissions.
func (d *Device) Root(ctx context.Context) error {
	res, err := d.Command(ctx, "root").Output(testexec.DumpLogOnError)
	if err != nil {
		return err
	}
	if strings.Contains(string(res), "cannot run as root") {
		return errors.New("adb root not available on this device")
	}
	return nil
}

// SetScreenOffTimeout sets the Android device's screen timeout. This function requires adb root access.
func (d *Device) SetScreenOffTimeout(ctx context.Context, t time.Duration) error {
	if err := d.Root(ctx); err != nil {
		return err
	}
	return d.ShellCommand(ctx, "settings", "put", "system", "screen_off_timeout", strconv.Itoa(int(t.Milliseconds()))).Run(testexec.DumpLogOnError)
}

// EnableBluetooth enables bluetooth on the Android device. This function requires adb root access.
func (d *Device) EnableBluetooth(ctx context.Context) error {
	if err := d.Root(ctx); err != nil {
		return err
	}
	return d.ShellCommand(ctx, "svc", "bluetooth", "enable").Run(testexec.DumpLogOnError)
}

// DisableBluetooth disables bluetooth on the Android device. This function requires adb root access.
func (d *Device) DisableBluetooth(ctx context.Context) error {
	if err := d.Root(ctx); err != nil {
		return err
	}
	return d.ShellCommand(ctx, "svc", "bluetooth", "disable").Run(testexec.DumpLogOnError)
}

// EnableBluetoothHciLogging enables verbose bluetooth HCI logging. This function requires adb root access.
func (d *Device) EnableBluetoothHciLogging(ctx context.Context) error {
	if err := d.Root(ctx); err != nil {
		return err
	}
	if err := d.ShellCommand(ctx, "setprop", "persist.bluetooth.btsnooplogmode", "full").Run(testexec.DumpLogOnError); err != nil {
		return err
	}
	// Restart bluetooth for the command to take effect.
	if err := d.DisableBluetooth(ctx); err != nil {
		return err
	}
	return d.EnableBluetooth(ctx)
}

// BluetoothMACAddress returns the Bluetooth MAC address.
func (d *Device) BluetoothMACAddress(ctx context.Context) (string, error) {
	mac, err := d.ShellCommand(ctx, "settings", "get", "secure", "bluetooth_address").Output(testexec.DumpLogOnError)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(mac), "\n"), nil
}

// EnableVerboseWifiLogging enables verbose WiFi logging on the device. This function requires adb root access.
func (d *Device) EnableVerboseWifiLogging(ctx context.Context) error {
	if err := d.Root(ctx); err != nil {
		return err
	}
	return d.ShellCommand(ctx, "cmd", "wifi", "set-verbose-logging", "enabled").Run(testexec.DumpLogOnError)
}
