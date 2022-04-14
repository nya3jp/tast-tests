package typec

import (
	"context"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
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

	s.Logf("START EXTERNAL USB STRESS TEST")

	svo.SetOnOff(ctx, servo.HubUSBReset, servo.Off)
	testing.Sleep(ctx, 10*time.Second)

	var lsusb []byte
	lsusb, err = d.Conn().CommandContext(ctx, "lsusb").Output()
	s.Logf("Servo Off lsusb: \n%s", lsusb)

	svo.SetOnOff(ctx, servo.HubUSBReset, servo.On)
        testing.Sleep(ctx, 20*time.Second)

        lsusb, err = d.Conn().CommandContext(ctx, "lsusb").Output()
        s.Logf("Servo On lsusb: \n%s", lsusb)

	s.Logf("END EXTERNAL USB STRESS TEST")
}
