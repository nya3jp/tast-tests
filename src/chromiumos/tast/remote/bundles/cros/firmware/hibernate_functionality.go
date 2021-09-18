package firmware

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/common/xmlrpc"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/security"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HibernateFunctionality,
		Desc:         "Verifies that system Hibernate functionality through onboard keyboard",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
		SoftwareDeps: []string{"chrome", "reboot"},
		Vars:         []string{"servo"},
		Attr:         []string{"group:firmware", "firmware_experimental"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
		Fixture:      fixture.NormalMode,
	})
}

func HibernateFunctionality(ctx context.Context, s *testing.State) {
	const (
		kbPressALT = "kbpress 10 0 1"
		kbPressH   = "kbpress 6 1 1"
		kbPressVOL = "kbpress 4 0 1"
	)
	dut := s.DUT()
	h := s.FixtValue().(*fixture.Value).Helper
	waitCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	getChargerPollOptions := testing.PollOptions{
		Timeout:  10 * time.Second,
		Interval: 250 * time.Millisecond,
	}
	s.Log("Stopping power supply")
	if err := h.SetDUTPower(ctx, false); err != nil {
		s.Fatal("Failed to remove charger: ", err)
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if attached, err := h.Servo.GetChargerAttached(ctx); err != nil {
			return err
		} else if !attached {
			return errors.New("charger is still attached - use Servo V4 Type-C or supply RPM vars")
		}
		return nil
	}, &getChargerPollOptions); err != nil {
		s.Fatal("Check for charger failed: ", err)
	}
	defer func(ctx context.Context) {
		if err := h.SetDUTPower(ctx, true); err != nil {
			s.Fatal("Failed to attach charger: ", err)
		}
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if attached, err := h.Servo.GetChargerAttached(ctx); err != nil {
				return err
			} else if !attached {
				return errors.New("charger is not attached")
			}
			return nil
		}, &getChargerPollOptions); err != nil {
			s.Fatal("Check for charger failed: ", err)
		}
		if !dut.Connected(ctx) {
			s.Log("Power Normal Pressing")
			if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
				s.Fatal("Failed to power button press: ", err)
			}
			if err := dut.WaitConnect(waitCtx); err != nil {
				s.Log("Failed to wake up DUT. Retrying")
				if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
					s.Fatal("Failed to power button press: ", err)
				}
				if err := dut.WaitConnect(waitCtx); err != nil {
					s.Fatal("Failed to wait connect DUT: ", err)
				}
			}
		}
	}(ctx)
	// Wait for a short delay between cutting power supply.
	if err := testing.Sleep(ctx, 2*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}
	cl, err := rpc.Dial(ctx, dut, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	//defer cl.Close(ctx)
	client := security.NewBootLockboxServiceClient(cl.Conn)
	if _, err := client.NewChromeLogin(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	if err := h.Servo.WatchdogRemove(ctx, servo.WatchdogCCD); err != nil {
		s.Fatal("Failed to remove CCD watchdog: ", err)
	}
	cl.Close(ctx)
	s.Log("Pressing ALT+H+VolUp to send the DUT to Hibernate")
	if err := h.Servo.RunECCommand(ctx, kbPressALT); err != nil {
		s.Fatal("Failed to press ALT key: ", err)
	}
	if err := h.Servo.RunECCommand(ctx, kbPressH); err != nil {
		s.Fatal("Failed to press H key: ", err)
	}
	if err := h.Servo.RunECCommand(ctx, kbPressVOL); err != nil {
		s.Fatal("Failed to press volume up key: ", err)
	}
	// Wait for a short delay after putting DUT in hibernation.
	if err := testing.Sleep(ctx, 4*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}
	s.Log("Checking DUT for Hibernate")
	if err := dut.WaitUnreachable(ctx); err != nil {
		s.Fatal("Failed to wait DUT to become unreachable: ", err)
	}

	// Verify that EC is non-responsive by querying an EC command.
	s.Log("Verify EC is non-responsive")
	waitECCtx, cancelEC := context.WithTimeout(ctx, 10*time.Second)
	defer cancelEC()

	// Expect no return for the query, and receive error of type FaultError.
	_, errEC := h.Servo.RunECCommandGetOutput(waitECCtx, "version", []string{`.`})
	if errEC == nil {
		s.Fatal(" Failed EC was still responsive after putting DUT in hibernation: ", err)
	}
	var errSend xmlrpc.FaultError
	if !errors.As(errEC, &errSend) {
		s.Fatal("Failed EC was still responsive after putting DUT in hibernation: ", errEC)
	}
	s.Log("EC was non-responsive")
	// Wait for DUT to reboot and reconnect.
	if err = h.SetDUTPower(ctx, true); err != nil {
		s.Fatal("Failed to connect charger: ", err)
	}
	// Wait for a short delay between cutting power supply and telling EC to hibernate.
	if err := testing.Sleep(ctx, 2*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}
	isAttached, err := h.Servo.GetChargerAttached(ctx)
	if err != nil {
		s.Fatal("Failed to check whether DUT is charging: ", err)
	}
	if !isAttached {
		s.Fatal("DUT is not charging after waking up from hibernation: ")
	}
	if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
		s.Fatal("Failed to power button press: ", err)
	}
	if err := dut.WaitConnect(waitCtx); err != nil {
		if err := h.Servo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
			s.Fatal("Failed to power button press: ", err)
		}
		if err := dut.WaitConnect(waitCtx); err != nil {
			s.Fatal("Failed to wait connect DUT: ", err)
		}
	}
}
