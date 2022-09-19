// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/hermes"
	"chromiumos/tast/local/modemfwd"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/upstart"
	cellular_pb "chromiumos/tast/services/cros/cellular"
	"chromiumos/tast/testing"
)

const (
	testServiceOrder    = "cellular,ethernet"
	defaultServiceOrder = "vpn,ethernet,wifi,cellular"
	defaultTimeout      = 30 * time.Second
	defaultLogLevel     = 2
)

var (
	defaultLogTags = []string{"cellular", "modem", "device"}
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			cellular_pb.RegisterRemoteCellularServiceServer(srv, &RemoteCellularService{s: s})
		},
	})
}

// RemoteCellularService implements tast.cros.cellular.RemoteCellularService.
type RemoteCellularService struct {
	s               *testing.ServiceState
	helper          *cellular.Helper
	modem           *modemmanager.Modem
	modemfwdStopped bool
}

// SetUp initialize the DUT for cellular_pb testing.
func (s *RemoteCellularService) SetUp(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	// Give some time for cellular daemons to perform any modem operations. Stopping them via upstart might leave the modem in a bad state.
	if err := cellular.EnsureUptime(ctx, 2*time.Minute); err != nil {
		return nil, errors.Wrap(err, "failed to wait for system uptime")
	}

	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create cellular.Helper")
	}
	s.helper = helper

	if _, err := helper.ResetModem(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to reset modem")
	}

	if s.modemfwdStopped, err = modemfwd.Stop(ctx); err != nil {
		return nil, errors.Wrapf(err, "failed to stop job: %q", modemfwd.JobName)
	}
	if s.modemfwdStopped {
		testing.ContextLogf(ctx, "Stopped %q", modemfwd.JobName)
	} else {
		testing.ContextLogf(ctx, "%q not running", modemfwd.JobName)
	}

	if err := helper.ResetShill(ctx); err != nil {
		return nil, errors.Wrap(err[len(err)-1], "failed to reset shill")
	}

	if err := helper.Manager.SetDebugLevel(ctx, defaultLogLevel); err != nil {
		return nil, errors.Wrap(err, "failed to set shill debug level")
	}

	if err := helper.Manager.SetDebugTags(ctx, defaultLogTags); err != nil {
		return nil, errors.Wrap(err, "failed to set shill debug tags")
	}

	if err := helper.RestartModemManager(ctx, true); err != nil {
		return nil, errors.Wrap(err, "failed to restart modem manager")
	}

	if err := helper.Manager.DisableTechnology(ctx, shill.TechnologyWifi); err != nil {
		return nil, errors.Wrap(err, "failed to disable wifi via shill")
	}

	if err := helper.Manager.SetServiceOrder(ctx, testServiceOrder); err != nil {
		return nil, errors.Wrap(err, "failed to change service order")
	}

	// make sure we are using the correct SIM
	if _, err = modemmanager.NewModemWithSim(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to find valid mm dbus object")
	}

	if _, err := helper.SetServiceAutoConnect(ctx, false); err != nil {
		return nil, errors.Wrap(err, "failed to turn off autoconnect")
	}

	// wait for hermes to stabilize
	if upstart.JobExists(ctx, "hermes") {
		if err := hermes.WaitForHermesIdle(ctx, 30*time.Second); err != nil {
			testing.ContextLog(ctx, "Could not confirm if Hermes is idle: ", err)
		}
	}
	return &empty.Empty{}, nil
}

// Reinit reinitializes the DUT for cellular testing between tests.
func (s *RemoteCellularService) Reinit(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	return &empty.Empty{}, nil
}

// TearDown releases any held resources and reverts the changes made in SetUp.
func (s *RemoteCellularService) TearDown(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if s.helper == nil {
		return nil, errors.New("Cellular helper not available, SetUp must be called first")
	}

	var allErrors error
	if _, err := s.helper.SetServiceAutoConnect(ctx, true); err != nil {
		allErrors = errors.Wrapf(allErrors, "failed to turn on autoconnect: %v", err) // NOLINT
	}

	if err := s.helper.Manager.SetServiceOrder(ctx, defaultServiceOrder); err != nil {
		allErrors = errors.Wrapf(allErrors, "failed to change service order: %v", err) // NOLINT
	}

	if err := s.helper.Manager.EnableTechnology(ctx, shill.TechnologyWifi); err != nil {
		allErrors = errors.Wrapf(allErrors, "failed to disable wifi via shill: %v", err) // NOLINT
	}

	if err := s.helper.ResetShill(ctx); err != nil {
		allErrors = errors.Wrapf(allErrors, "failed to reset shill: %v", err) // NOLINT
	}

	if err := s.helper.RestartModemManager(ctx, false); err != nil {
		allErrors = errors.Wrapf(allErrors, "failed to restart modem manager: %v", err) // NOLINT
	}

	if _, err := s.helper.ResetModem(ctx); err != nil {
		allErrors = errors.Wrap(err, "failed to reset modem") // NOLINT
	}

	if s.modemfwdStopped {
		if err := modemfwd.StartAndWaitForQuiescence(ctx); err != nil {
			allErrors = errors.Wrap(err, "failed to restart modemfwd") // NOLINT
		}
		testing.ContextLogf(ctx, "Started %q", modemfwd.JobName)
	}

	return &empty.Empty{}, allErrors
}

// Enable enables the cellular technology on the DUT.
func (s *RemoteCellularService) Enable(ctx context.Context, req *empty.Empty) (*cellular_pb.EnableResponse, error) {
	if s.helper == nil {
		return &cellular_pb.EnableResponse{}, errors.New("Cellular helper not available, SetUp must be called first")
	}

	elapsed, err := s.helper.Enable(ctx)
	if err != nil {
		return nil, err
	}

	return &cellular_pb.EnableResponse{
		EnableTime: int64(elapsed),
	}, nil
}

// Disable disables the cellular technology on the DUT.
func (s *RemoteCellularService) Disable(ctx context.Context, req *empty.Empty) (*cellular_pb.DisableResponse, error) {
	if s.helper == nil {
		return nil, errors.New("Cellular helper not available, SetUp must be called first")
	}

	// make sure DUT is not connected to callbox before disabling as it sometimes causes issues reattaching later
	if err := s.helper.IsConnected(ctx); err == nil {
		if _, err := s.helper.Disconnect(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to disable cellular, unable to disconnect from current network")
		}
	}

	elapsed, err := s.helper.Disable(ctx)
	if err != nil {
		return nil, err
	}

	return &cellular_pb.DisableResponse{
		DisableTime: int64(elapsed),
	}, nil
}

// Connect attempts to connect to the cellular service.
func (s *RemoteCellularService) Connect(ctx context.Context, req *empty.Empty) (*cellular_pb.ConnectResponse, error) {
	if s.helper == nil {
		return nil, errors.New("Cellular helper not available, SetUp must be called first")
	}

	elapsed, err := s.helper.ConnectToDefault(ctx)
	if err != nil {
		return nil, err
	}

	return &cellular_pb.ConnectResponse{
		ConnectTime: int64(elapsed),
	}, nil
}

// Disconnect attempts to disconnect from the cellular service.
func (s *RemoteCellularService) Disconnect(ctx context.Context, req *empty.Empty) (*cellular_pb.DisconnectResponse, error) {
	if s.helper == nil {
		return nil, errors.New("Cellular helper not available, SetUp must be called first")
	}

	elapsed, err := s.helper.Disconnect(ctx)
	if err != nil {
		return nil, err
	}

	return &cellular_pb.DisconnectResponse{
		DisconnectTime: int64(elapsed),
	}, nil
}

// QueryService returns the cellular service interface properties.
func (s *RemoteCellularService) QueryService(ctx context.Context, _ *empty.Empty) (*cellular_pb.QueryServiceResponse, error) {
	if s.helper == nil {
		return nil, errors.New("Cellular helper not available, SetUp must be called first")
	}

	service, err := s.helper.FindServiceForDevice(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get cellular service")
	}

	props, err := service.GetProperties(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get service properties")
	}

	name, err := props.GetString(shillconst.ServicePropertyName)
	if err != nil {
		return nil, err
	}
	device, err := props.GetObjectPath(shillconst.ServicePropertyDevice)
	if err != nil {
		return nil, err
	}
	state, err := props.GetString(shillconst.ServicePropertyState)
	if err != nil {
		return nil, err
	}
	isConnected, err := props.GetBool(shillconst.ServicePropertyIsConnected)
	if err != nil {
		return nil, err
	}

	return &cellular_pb.QueryServiceResponse{
		Name:        name,
		Device:      string(device),
		State:       state,
		IsConnected: isConnected,
	}, nil
}

// QueryInterface returns the cellular device interface properties.
func (s *RemoteCellularService) QueryInterface(ctx context.Context, _ *empty.Empty) (*cellular_pb.QueryInterfaceResponse, error) {
	if s.helper == nil {
		return nil, errors.New("Cellular helper not available, SetUp must be called first")
	}

	props, err := s.helper.Device.GetProperties(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get device properties")
	}

	iface, err := props.GetString(shillconst.DevicePropertyInterface)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get device interface from properties")
	}

	return &cellular_pb.QueryInterfaceResponse{
		Name: iface,
	}, nil
}

// WaitForNextSms waits until a single sms added signal is received.
func (s *RemoteCellularService) WaitForNextSms(ctx context.Context, _ *empty.Empty) (*cellular_pb.WaitForNextSmsResponse, error) {
	match := dbusutil.MatchSpec{
		Type:      "signal",
		Interface: modemmanager.DBusModemmanagerMessageInterface,
		Member:    "Added",
	}

	conn, err := dbusutil.SystemBus()
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to system bus")
	}

	signal, err := dbusutil.GetNextSignal(ctx, conn, match)
	if err != nil {
		return nil, errors.Wrap(err, "failed to receive SMS Added signal")
	}

	message, err := smsFromSignal(ctx, signal)
	if err != nil {
		return nil, errors.Wrap(err, "faild to get SMS message from signal")
	}
	return &cellular_pb.WaitForNextSmsResponse{Message: message}, nil
}

func smsFromSignal(ctx context.Context, signal *dbus.Signal) (*cellular_pb.SmsMessage, error) {
	if len(signal.Body) < 1 {
		return nil, errors.New("SMS signal body empty")
	}

	smsPath, ok := signal.Body[0].(dbus.ObjectPath)
	if !ok {
		return nil, errors.Errorf("failed to get SMS path from signal %v", signal.Body)
	}
	ph, err := dbusutil.NewPropertyHolder(ctx, modemmanager.DBusModemmanagerService, modemmanager.DBusModemmanagerSmsInterface, smsPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create SMS property holder")
	}

	props, err := ph.GetProperties(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get SMS properties")
	}

	text, err := props.GetString("Text")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get SMS text")
	}

	return &cellular_pb.SmsMessage{
		Text: text,
	}, nil
}
