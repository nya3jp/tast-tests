package power

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/common/xmlrpc"
	"chromiumos/tast/errors"
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
		SoftwareDeps: []string{"reboot", "chrome"},
		Vars:         []string{"servo"},
		ServiceDeps:  []string{"tast.cros.security.BootLockboxService"},
		Attr:         []string{"group:mainline", "informational"},
		HardwareDeps: hwdep.D(hwdep.ChromeEC()),
	})
}

func HibernateFunctionality(ctx context.Context, s *testing.State) {
	const (
		kbPressALT = "kbpress 10 0 1"
		kbPressH   = "kbpress 6 1 1"
		kbPressVOL = "kbpress 4 0 1"
	)
	dut := s.DUT()
	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)
	if err = pxy.Servo().SetPDRole(ctx, servo.PDRoleSnk); err != nil {
		s.Fatal("Failed to disconnect charger: ", err)
	}
	defer func(ctx context.Context) {
		isAttached, err := pxy.Servo().GetChargerAttached(ctx)
		if err != nil {
			s.Fatal("Failed to get charger status: ", err)
		}
		if !isAttached {
			if err = pxy.Servo().SetPDRole(ctx, servo.PDRoleSnk); err != nil {
				s.Fatal("Failed to check whether DUT is charging: ", err)
			}
		}
		if !dut.Connected(ctx) {
			if err := pxy.Servo().SetString(ctx, "power_key", "press"); err != nil {
				s.Fatal("Failed to power state on: ", err)
			}
			if err := dut.WaitConnect(ctx); err != nil {
				s.Log("Unable to wake up DUT. Retrying")
				if err := pxy.Servo().SetString(ctx, "power_key", "press"); err != nil {
					s.Fatal("Failed to power state on: ", err)
				}
				if err := dut.WaitConnect(ctx); err != nil {
					s.Fatal("Failed to wait connect DUT: ", err)
				}
			}
		}
	}(ctx)
	// Wait for a short delay between cutting power supply.
	if err := testing.Sleep(ctx, 2*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}
	ok, err := pxy.Servo().GetChargerAttached(ctx)
	if err != nil {
		s.Fatal("Failed to check whether power is off: ", err)
	}
	if ok {
		s.Fatal("Failed power was still on after disabling servo charge-through")
	}
	cl, err := rpc.Dial(ctx, dut, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	client := security.NewBootLockboxServiceClient(cl.Conn)
	if _, err := client.NewChromeLogin(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	s.Log("Pressing ALT+H+VolUp to send the DUT to Hibernate")
	if err := pxy.Servo().RunECCommand(ctx, kbPressALT); err != nil {
		s.Fatal("Failed to press ALT key: ", err)
	}
	if err := pxy.Servo().RunECCommand(ctx, kbPressH); err != nil {
		s.Fatal("Failed to press H key: ", err)
	}
	if err := pxy.Servo().RunECCommand(ctx, kbPressVOL); err != nil {
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
	_, errEC := pxy.Servo().RunECCommandGetOutput(waitECCtx, "version", []string{`.`})
	if errEC == nil {
		s.Fatal(" Failed EC was still responsive after putting DUT in hibernation: ", err)
	}
	var errSend xmlrpc.FaultError
	if !errors.As(errEC, &errSend) {
		s.Fatal("Failed EC was still responsive after putting DUT in hibernation: ", errEC)
	}
	s.Log("EC was non-responsive")
	// Wait for DUT to reboot and reconnect.
	if err = pxy.Servo().SetPDRole(ctx, servo.PDRoleSrc); err != nil {
		s.Fatal("Failed to connect charger: ", err)
	}
	// Wait for a short delay between cutting power supply and telling EC to hibernate.
	if err := testing.Sleep(ctx, 2*time.Second); err != nil {
		s.Fatal("Failed to sleep: ", err)
	}
	isAttached, err := pxy.Servo().GetChargerAttached(ctx)
	if err != nil {
		s.Fatal("Failed to check whether DUT is charging: ", err)
	}
	if !isAttached {
		s.Fatal("DUT is not charging after waking up from hibernation: ")
	}
	if err := pxy.Servo().SetString(ctx, "power_key", "press"); err != nil {
		s.Fatal("Failed to power normal press: ", err)
	}
	if err := dut.WaitConnect(ctx); err != nil {
		if err := pxy.Servo().SetString(ctx, "power_key", "press"); err != nil {
			s.Fatal("Failed to power normal press: ", err)
		}
		if err := dut.WaitConnect(ctx); err != nil {
			s.Fatal("Failed to wait connect DUT: ", err)
		}
	}
}
