package typec

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/testing"
)

const waitDelay = 10
const longTimeout = 60
const repeatTest = 10

func init() {
	testing.AddTest(&testing.Test{
		Func:     ExternalUSBStress,
		Desc:     "",
		Contacts: []string{"wonchung@google.com", "chromeos-usb@google.com"},
		Attr:     []string{"group:mainline", "group:typec", "informational"},
		Vars:     []string{"servo"},
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

	logsBefore := GetUSBLogs(ctx, s, d)
        SetHubPower(ctx, svo, false)
	testing.Sleep(ctx, waitDelay*time.Second)
        SetHubPower(ctx, svo, true)
	testing.Sleep(ctx, 2*waitDelay*time.Second)
        logsAfter := GetUSBLogs(ctx, s, d)
        if !CheckUSBDisconnectAndConnect(logsBefore, logsAfter, s) {
                s.Fatal("Could not detect USB device disconnect/connect")
        }

	for i := 0; i < repeatTest; i++ {
		TestStress(ctx, s, svo, d)
	}
}

func TestStress(ctx context.Context, s *testing.State, svo *servo.Servo, d *dut.DUT) {
	TestHotplug(ctx, svo)

	TestSuspend(ctx, s, svo, d, false, false)

	TestSuspend(ctx, s, svo, d, true, false)

	TestSuspend(ctx, s, svo, d, false, true)

	TestSuspend(ctx, s, svo, d, true, true)
}

func TestSuspend(ctx context.Context, s *testing.State, svo *servo.Servo, d *dut.DUT, pluggedBeforeSuspend bool, pluggedBeforeResume bool) {
	var logsBefore, logsAfter []string

	SetHubPower(ctx, svo, true)
	logsBefore := GetUSBLogs(ctx, s, d)

	SetHubPower(ctx, svo, pluggedBeforeSuspend)
	svo.CloseLid(ctx)

	SetHubPower(ctx, svo, pluggedBeforeResume)
	svo.OpenLid(ctx)

	SetHubPower(ctx, svo, true)
	logsAfter = GetUSBLogs(ctx, s, d)

	if !CheckUSBDisconnectAndConnect(logsBefore, logsAfter, s) {
		s.Fatal("Could not detect USB device disconnect/connect")
	}
}

func TestHotplug(ctx context.Context, svo *servo.Servo) {
	SetHubPower(ctx, svo, false)
	SetHubPower(ctx, svo, true)
	testing.Sleep(ctx, longTimeout*time.Second)
}

func SetHubPower(ctx context.Context, svo *servo.Servo, on bool) {
	if on {
		// Reset "Off" turns on the hub
		svo.SetOnOff(ctx, servo.HubUSBReset, servo.Off)
	} else {
		// Reset "On" turns off the hub
		svo.SetOnOff(ctx, servo.HubUSBReset, servo.On)
	}
	testing.Sleep(ctx, waitDelay*time.Second)
}

func GetUSBLogs(ctx context.Context, s *testing.State, d *dut.DUT) []string {
	log, err := d.Conn().CommandContext(ctx, "grep", "usb", "/var/log/messages").Output()
	if err != nil {
		s.Fatal("Failed to read USB logs: ", err)
	}
	return strings.Split(string(log), "\n")
}

func CheckUSBDisconnectAndConnect(logsBefore []string, logsAfter []string, s *testing.State) bool {
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

	var disconnected, connected bool = false, false
	for _, l := range logsDiff {
		if strings.Contains(l, "USB disconnect") {
			s.Logf("%s", l)
			disconnected = true
		} else if strings.Contains(l, "New USB device found") {
			s.Logf("%s",l)
			connected = true
		}
	}

	return disconnected && connected
}
