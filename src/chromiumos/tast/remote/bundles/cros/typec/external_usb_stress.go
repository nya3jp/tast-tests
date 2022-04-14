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

	s.Logf("START EXTERNAL USB STRESS TEST\n")

	svo.SetUSBMuxState(ctx, servo.USBMuxDUT)

	Tmp(ctx, s, svo, d)

	s.Logf("END EXTERNAL USB STRESS TEST\n")
}

func difference(a, b []string) []string {
	mb := make(map[string]struct{}, len(b))
	for _, x := range b {
		mb[x] = struct{}{}
	}
	var diff []string
	for _, x := range a {
		if _, found := mb[x]; !found {
			diff = append(diff, x)
		}
	}
	return diff
}

func Tmp(ctx context.Context, s *testing.State, svo *servo.Servo, d *dut.DUT) {
	msg1, err := d.Conn().CommandContext(ctx, "grep", "usb", "/var/log/messages").Output()
	if err != nil {
		s.Logf("error on grep 1")
	}

	svo.SetOnOff(ctx, servo.HubUSBReset, servo.On)  // "On" turns off the hub
        s.Logf("hub OFF\n")
        testing.Sleep(ctx, 10*time.Second)

	svo.SetOnOff(ctx, servo.HubUSBReset, servo.Off) // "Off" turns on the hub
	s.Logf("hub ON\n")
	testing.Sleep(ctx, 10*time.Second)

	msg2, err := d.Conn().CommandContext(ctx, "grep", "usb", "/var/log/messages").Output()
	if err != nil {
		s.Logf("error on grep 2")
	}

	list1 := strings.Split(string(msg1), "\n")
	list2 := strings.Split(string(msg2), "\n")

	difflist := difference(list2, list1)

	for _, val := range difflist {
		if strings.Contains(val, "USB disconnect") {
			s.Logf(val)
		}
		if strings.Contains(val, "New USB device found") {
			s.Logf(val)
		}
	}
}
