// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typec

import (
	"context"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/testing"
)

const waitDelay = 10
const defaultIterations = 10

func init() {
	testing.AddTest(&testing.Test{
		Func:     ExternalUSBStress,
		Desc:     "Checks if USB changes can be detected correctly",
		Contacts: []string{"wonchung@google.com", "chromeos-usb@google.com"},
		Attr:     []string{"group:mainline", "group:typec", "informational"},
		Vars:     []string{"servo", "typec.iterations"},
	})
}

func ExternalUSBStress(ctx context.Context, s *testing.State) {
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	d := s.DUT()
	if !d.Connected(ctx) {
		s.Fatal("Failed DUT connection check at the beginning")
	}

	servoSpec, _ := s.Var("servo")
	pxy, err := servo.NewProxy(ctx, servoSpec, d.KeyFile(), d.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctxForCleanUp)

	svo := pxy.Servo()
	svo.SetUSBMuxState(ctx, servo.USBMuxDUT)

	logsBefore := getUSBLogs(ctx, s, d)
	setHubPower(ctx, s, svo, false)
	testing.Sleep(ctx, waitDelay*time.Second)
	setHubPower(ctx, s, svo, true)
	testing.Sleep(ctx, 2*waitDelay*time.Second)
	logsAfter := getUSBLogs(ctx, s, d)
	if !checkUSBConnect(logsBefore, logsAfter, true) {
		s.Fatal("Failed to detect USB device connect")
	}

	iterations := defaultIterations
	customIterations, ok := s.Var("typec.iterations")
	if ok {
		iterations, _ = strconv.Atoi(customIterations)
	}

	emptyLogs(ctx, d)
	for i := 0; i < iterations; i++ {
		testStress(ctx, s, svo, d)
	}
}

func testStress(ctx context.Context, s *testing.State, svo *servo.Servo, d *dut.DUT) {
	testHotplug(ctx, s, svo)

	testSuspend(ctx, s, svo, d, false, false)

	testSuspend(ctx, s, svo, d, true, false)

	testSuspend(ctx, s, svo, d, false, true)

	testSuspend(ctx, s, svo, d, true, true)
}

// testSuspend connects/disconnects USB hub to DUT while closing/opening DUT lid.
func testSuspend(ctx context.Context,
	s *testing.State,
	svo *servo.Servo,
	d *dut.DUT,
	pluggedBeforeSuspend,
	pluggedBeforeResume bool) {
	// Turn servo on before retrieving USB logs since DUT may be connected
	// to the network through servo
	setHubPower(ctx, s, svo, true)
	logsBefore := getUSBLogs(ctx, s, d)

	setHubPower(ctx, s, svo, pluggedBeforeSuspend)
	svo.CloseLid(ctx)
	testing.Sleep(ctx, waitDelay*time.Second)

	setHubPower(ctx, s, svo, pluggedBeforeResume)
	svo.OpenLid(ctx)
	testing.Sleep(ctx, waitDelay*time.Second)

	// Turn servo on before retrieving USB logs since DUT may be connected
	// to the network through servo
	setHubPower(ctx, s, svo, true)
	// Reconnect to DUT since it has been suspended and resumed
	err := d.WaitConnect(ctx)
	if err != nil {
		s.Fatal("Failed to reconnect to DUT")
	}
	testing.Sleep(ctx, waitDelay*time.Second)
	logsAfter := getUSBLogs(ctx, s, d)

	disconnectedDuringSuspend := pluggedBeforeSuspend && !pluggedBeforeResume
	noUSBAction := pluggedBeforeSuspend && pluggedBeforeResume
	if !noUSBAction && !checkUSBConnect(logsBefore, logsAfter, !disconnectedDuringSuspend) {
		s.Fatal("Failed to detect USB device connect")
	}
}

// testHotplug disconnects and connects USB hub from DUT.
func testHotplug(ctx context.Context, s *testing.State, svo *servo.Servo) {
	setHubPower(ctx, s, svo, false)
	setHubPower(ctx, s, svo, true)
	testing.Sleep(ctx, waitDelay*time.Second)
}

// setHubPower turns on or off the USB hub. (dut_hub1_rst1)
func setHubPower(ctx context.Context, s *testing.State, svo *servo.Servo, on bool) {
	if on {
		// Reset "Off" turns on the hub
		svo.SetOnOff(ctx, servo.HubUSBReset, servo.Off)
		s.Log("USB device connected to DUT")
	} else {
		// Reset "On" turns off the hub
		svo.SetOnOff(ctx, servo.HubUSBReset, servo.On)
		s.Log("USB device disconnected from DUT")
	}
	testing.Sleep(ctx, waitDelay*time.Second)
}

// getUSBLogs filters and gives USB related logs, like USB device disconnect/connect.
func getUSBLogs(ctx context.Context, s *testing.State, d *dut.DUT) []string {
	log, err := d.Conn().CommandContext(ctx, "grep", "usb", "/var/log/messages").Output()
	if err != nil {
		s.Fatal("Failed to read USB logs: ", err)
	}
	return strings.Split(string(log), "\n")
}

// emptyLogs clears unnecessary logs before checking for USB logs.
func emptyLogs(ctx context.Context, d *dut.DUT) {
	d.Conn().CommandContext(ctx, "sh", "-c", "echo > /var/log/messages").Run()
}

// checkUSBConnect compares two logs (before and after) and return true if
// there exists a new USB device connect log.
func checkUSBConnect(logsBefore, logsAfter []string, shouldHaveDisconnectLog bool) bool {
	mLogsBefore := make(map[string]struct{}, len(logsBefore))
	for _, l := range logsBefore {
		mLogsBefore[l] = struct{}{}
	}
	var logsDiff []string
	for _, l := range logsAfter {
		if _, found := mLogsBefore[l]; !found {
			logsDiff = append(logsDiff, l)
		}
	}

	connected := false
	disconnected := !shouldHaveDisconnectLog
	for _, l := range logsDiff {
		if strings.Contains(l, "New USB device found") {
			connected = true
		}
		if shouldHaveDisconnectLog && strings.Contains(l, "USB disconnect") {
			disconnected = true
		}
	}

	return disconnected && connected
}
