// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package fixture implements fixtures for firmware tests.
package fixture

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"

	common "chromiumos/tast/common/firmware"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	pb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/testing"
)

// Fixture names for the tests to use.
const (
	NormalMode              = "bootModeNormal"
	DevMode                 = "bootModeDev"
	DevModeGBB              = "bootModeDevGBB"
	USBDevModeNoServices    = "bootModeUSBDevNoServices"
	USBDevModeGBBNoServices = "bootModeUSBDevGBBNoServices"
	RecModeNoServices       = "bootModeRecNoServices"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            NormalMode,
		Desc:            "Reboot into normal mode before test",
		Contacts:        []string{"tast-fw-library-reviewers@google.com", "jbettis@google.com"},
		Impl:            newFixture(common.BootModeNormal, false, true),
		Vars:            []string{"servo", "dutHostname", "powerunitHostname", "powerunitOutlet", "hydraHostname", "firmware.no_ec_sync", "firmware.skipFlashUSB", "noSSH"},
		SetUpTimeout:    10 * time.Second,
		ResetTimeout:    10 * time.Second,
		PreTestTimeout:  5 * time.Minute,
		PostTestTimeout: 5 * time.Minute,
		TearDownTimeout: 5 * time.Minute,
		Data:            []string{firmware.ConfigFile},
	})
	testing.AddFixture(&testing.Fixture{
		Name:            DevMode,
		Desc:            "Reboot into dev mode before test",
		Contacts:        []string{"tast-fw-library-reviewers@google.com", "jbettis@google.com"},
		Impl:            newFixture(common.BootModeDev, false, true),
		Vars:            []string{"servo", "dutHostname", "powerunitHostname", "powerunitOutlet", "hydraHostname", "firmware.no_ec_sync", "firmware.skipFlashUSB", "noSSH"},
		SetUpTimeout:    10 * time.Second,
		ResetTimeout:    10 * time.Second,
		PreTestTimeout:  5 * time.Minute,
		PostTestTimeout: 5 * time.Minute,
		TearDownTimeout: 5 * time.Minute,
		Data:            []string{firmware.ConfigFile},
	})
	testing.AddFixture(&testing.Fixture{
		Name:            DevModeGBB,
		Desc:            "Reboot into dev mode using GBB flags before test",
		Contacts:        []string{"tast-fw-library-reviewers@google.com", "jbettis@google.com"},
		Impl:            newFixture(common.BootModeDev, true, true),
		Vars:            []string{"servo", "dutHostname", "powerunitHostname", "powerunitOutlet", "hydraHostname", "firmware.no_ec_sync", "firmware.skipFlashUSB", "noSSH"},
		SetUpTimeout:    10 * time.Second,
		ResetTimeout:    10 * time.Second,
		PreTestTimeout:  5 * time.Minute,
		PostTestTimeout: 5 * time.Minute,
		TearDownTimeout: 5 * time.Minute,
		Data:            []string{firmware.ConfigFile},
	})
	testing.AddFixture(&testing.Fixture{
		Name:            USBDevModeNoServices,
		Desc:            "Reboot into usb-dev mode before test, ServiceDeps are not supported",
		Contacts:        []string{"tast-fw-library-reviewers@google.com", "jbettis@google.com"},
		Impl:            newFixture(common.BootModeUSBDev, false, false),
		Vars:            []string{"servo", "dutHostname", "powerunitHostname", "powerunitOutlet", "hydraHostname", "firmware.no_ec_sync", "firmware.skipFlashUSB", "noSSH"},
		SetUpTimeout:    60 * time.Minute, // Setting up USB key is slow
		ResetTimeout:    10 * time.Second,
		PreTestTimeout:  5 * time.Minute,
		PostTestTimeout: 5 * time.Minute,
		TearDownTimeout: 5 * time.Minute,
		Data:            []string{firmware.ConfigFile},
	})
	testing.AddFixture(&testing.Fixture{
		Name:            USBDevModeGBBNoServices,
		Desc:            "Reboot into usb-dev mode using GBB flags before test, ServiceDeps are not supported",
		Contacts:        []string{"tast-fw-library-reviewers@google.com", "jbettis@google.com"},
		Impl:            newFixture(common.BootModeUSBDev, true, false),
		Vars:            []string{"servo", "dutHostname", "powerunitHostname", "powerunitOutlet", "hydraHostname", "firmware.no_ec_sync", "firmware.skipFlashUSB", "noSSH"},
		SetUpTimeout:    60 * time.Minute, // Setting up USB key is slow
		ResetTimeout:    10 * time.Second,
		PreTestTimeout:  5 * time.Minute,
		PostTestTimeout: 5 * time.Minute,
		TearDownTimeout: 5 * time.Minute,
		Data:            []string{firmware.ConfigFile},
	})
	testing.AddFixture(&testing.Fixture{
		Name:            RecModeNoServices,
		Desc:            "Reboot into recovery mode before test, ServiceDeps are not supported",
		Contacts:        []string{"tast-fw-library-reviewers@google.com", "jbettis@google.com"},
		Impl:            newFixture(common.BootModeRecovery, false, false),
		Vars:            []string{"servo", "dutHostname", "powerunitHostname", "powerunitOutlet", "hydraHostname", "firmware.no_ec_sync", "firmware.skipFlashUSB", "noSSH"},
		SetUpTimeout:    60 * time.Minute, // Setting up USB key is slow
		ResetTimeout:    10 * time.Second,
		PreTestTimeout:  5 * time.Minute,
		PostTestTimeout: 5 * time.Minute,
		TearDownTimeout: 5 * time.Minute,
		Data:            []string{firmware.ConfigFile},
	})
}

// Value contains fields that are useful for tests.
type Value struct {
	BootMode      common.BootMode
	GBBFlags      pb.GBBFlagsState
	Helper        *firmware.Helper
	ForcesDevMode bool
}

// impl contains fields that are useful for Fixture methods.
type impl struct {
	value         *Value
	origBootMode  *common.BootMode
	origGBBFlags  *pb.GBBFlagsState
	copyTastFiles bool
	disallowSSH   bool
}

func networkInterfaceDetails(ctx context.Context) {
	var count int

	ifaces, err := net.Interfaces()
	if err != nil {
		testing.ContextLog(ctx, "localAddresses:", err.Error())
		return
	}

	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			testing.ContextLog(ctx, "localAddresses:", err.Error())
			continue
		}

		for _, a := range addrs {
			testing.ContextLogf(ctx, "Index:%d Name:%v addr:%v, mac:%v\n", i.Index, i.Name, a, i.HardwareAddr)
			if strings.Contains(i.Flags.String(), "up") {
				fmt.Println("Status : UP")
			} else {
				fmt.Println("Status : DOWN")
			}
		}
		count++
	}
	testing.ContextLog(ctx, "Total interfaces : ", count)
}

// func dockerClientLocal(ctx context.Context) (*client.Client, error) {
func dockerClientLocal() (*client.Client, error) {
	// Create Docker Client.
	// If the dockerd socket exists, use the default option.
	// Otherwise, try to use the tcp connection local host IP 192.168.231.1:2375
	// for satlab device.

	// testing.ContextLog(ctx, "DOCKER_HOST:", os.Getenv("DOCKER_HOST"))
	// testing.ContextLog(ctx, "DOCKER_API_VERSION: ", os.Getenv("DOCKER_API_VERSION"))
	// os.Setenv("DOCKER_HOST", "tcp://192.168.231.1:2375")

	// testing.ContextLog(ctx, "DOCKER_API_VERSION: ", os.Getenv("DOCKER_API_VERSION"))
	if _, err := os.Stat("/var/run/docker.sock"); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			// testing.ContextLog(ctx, "dockerClientLocal: os.Stat /var/run/docker.sock", err)
			return nil, err
		}
		// testing.ContextLog(ctx, "dockerClientLocal:/var/run/docker.sock not found, creating TCP connect")
		// testing.ContextLog(ctx, "DOCKER_HOST:", os.Getenv("DOCKER_HOST"))
		// testing.ContextLog(ctx, "DOCKER_API_VERSION: ", os.Getenv("DOCKER_API_VERSION"))
		// os.Setenv("DOCKER_HOST", "tcp://192.168.231.1:2375")

		return client.NewClientWithOpts(client.WithHost("tcp://192.168.231.1:2375"), client.WithAPIVersionNegotiation())
	}
	return client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
}

// newFixture creates an instance of firmware Fixture.
func newFixture(mode common.BootMode, forceDev, copyTastFiles bool) testing.FixtureImpl {
	return &impl{
		value: &Value{
			BootMode:      mode,
			ForcesDevMode: forceDev,
		},
		copyTastFiles: copyTastFiles,
	}
}

func (i *impl) noECSync(s *testing.FixtState) (bool, error) {
	noECSync := false
	noECSyncStr, ok := s.Var("firmware.no_ec_sync")
	if ok {
		var err error
		noECSync, err = strconv.ParseBool(noECSyncStr)
		if err != nil {
			return false, errors.Errorf("invalid value for var firmware.no_ec_sync: got %q, want true/false", noECSyncStr)
		}
	}
	return noECSync, nil
}

func runCmd(ctx context.Context, cmd_value string, arg ...string) {
	out, err := testexec.CommandContext(ctx, cmd_value, arg...).Output()
	if err != nil {
		testing.ContextLog(ctx, "Error runCmd output", cmd_value, err)
	}
	testing.ContextLog(ctx, "runCmd output", cmd_value, string(out))
}

// SetUp is called by the framework to set up the environment with possibly heavy-weight
// operations.
func (i *impl) SetUp(ctx context.Context, s *testing.FixtState) interface{} {

	noSSHVar, ok := s.Var("noSSH")
	i.disallowSSH = false
	if ok {
		forbidSSH, err := strconv.ParseBool(noSSHVar)
		if err != nil {
			s.Fatalf("Invalid value for var noSSH: got %q, want true/false", noSSHVar)
		}
		i.disallowSSH = forbidSSH
	}

	// buildInfo, ok := debug.ReadBuildInfo()
	// if !ok {
	// 	s.Log("Can't read BuildInfo")
	// }
	// s.Log("Dependencies:")
	// for _, dep := range buildInfo.Deps {
	// 	s.Logf("  %s %s\n", dep.Path, dep.Version)
	// }

	// host := "192.168.231.1"
	// port := "2375"
	timeout := time.Duration(1 * time.Second)

	// c, err := net.DialTimeout("tcp", host+":"+port, timeout)
	// if err != nil {
	// 	s.Log("not responding", err.Error())
	// } else {
	// 	s.Log("Responding", c)
	// }
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: timeout,
		}).DialContext,
	}
	cnt := http.Client{Transport: transport}

	// ctx1 := context.Background()
	// ctx1, cancel1 := context.WithTimeout(ctx1, time.Minute*5)
	// defer cancel1()

	// networkInterfaceDetails(ctx1)
	// runCmd(ctx1, "pwd")
	// runCmd(ctx1, "ls", "-alt", "/")
	// runCmd(ctx1, "ps", "-elf")
	// runCmd(ctx1, "netstat")
	// runCmd(ctx1, "printenv")

	s.Log("Context", ctx)

	// s.Log("Context1", ctx1)
	// cli, err := dockerClientLocal()

	// helper, err := connhelper.GetConnectionHelper("tcp://192.168.231.1:2375")
	// if err != nil {
	// 	s.Log("connhelper", err)
	// }
	cli, err := client.NewClientWithOpts(client.WithHost("tcp://192.168.231.1:2375"), client.WithHTTPClient(&cnt), client.WithAPIVersionNegotiation())
	if err != nil {
		s.Log("dockerClientLocal", err)
	} else {
		s.Log("dockerClientLocal call pass")
	}

	testing.ContextLog(ctx, "dockerClientLocal:CLD:", cli)
	s.Log(cli.ClientVersion())
	s.Log(cli.DaemonHost())
	s.Log(cli.HTTPClient())

	dl, ok := ctx.Deadline()
	if !ok {
		timeLeft := 0
		testing.ContextLog(ctx, "timeout left Before Ping: ", timeLeft)
	} else {
		timeLeft := time.Until(dl)
		testing.ContextLog(ctx, "timeout left Before Ping: ", timeLeft)
	}

	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		s.Fatal("ContainerList: ", err)
	}
	for _, container := range containers {
		s.Log(container)
	}

	ping, err := cli.Ping(context.Background())
	if err != nil {
		testing.ContextLog(ctx, "dockerClientLocal:Failed to Ping", err)
		s.Fatal("Ping Failed: ", err)
	}
	testing.ContextLog(ctx, "dockerClientLocal: Sucess Ping .......", ping)

	dla, ok := ctx.Deadline()
	if !ok {
		timeLeft := 0
		testing.ContextLog(ctx, "timeout left After Ping: ", timeLeft)
	} else {
		timeLeft := time.Until(dla)
		testing.ContextLog(ctx, "timeout left After Ping", timeLeft)
	}

	info, err := cli.Info(ctx)
	if err != nil {
		s.Fatal("failed to get docker info: ", err)
	}
	s.Log(info)
	s.Log("Creating a new firmware Helper instance for fixture: ", i.String())

	i.initHelper(ctx, s)

	if i.disallowSSH {
		s.Log("Skipping GBB and reboot because noSSH var was set")
		return i.value
	}

	flags := pb.GBBFlagsState{Clear: common.AllGBBFlags(), Set: common.FAFTGBBFlags()}
	if i.value.ForcesDevMode {
		if i.value.BootMode == common.BootModeUSBDev {
			common.GBBAddFlag(&flags, pb.GBBFlag_FORCE_DEV_SWITCH_ON, pb.GBBFlag_DEV_SCREEN_SHORT_DELAY, pb.GBBFlag_FORCE_DEV_BOOT_USB)
		} else {
			common.GBBAddFlag(&flags, pb.GBBFlag_FORCE_DEV_SWITCH_ON, pb.GBBFlag_DEV_SCREEN_SHORT_DELAY)
		}
	}
	noECSync, err := i.noECSync(s)
	if err != nil {
		s.Fatal("ECSync: ", err)
	}
	if noECSync {
		common.GBBAddFlag(&flags, pb.GBBFlag_DISABLE_EC_SOFTWARE_SYNC)
		s.Log("User selected to disable EC software sync")
	}
	i.value.GBBFlags = flags

	if err := firmware.DockerClientPing(ctx); err != nil {
		s.Fatal("Failed Docker Daemon Ping Before InitHelper: ", err)
	}
	s.Log("Ping Success Before InitHelper")

	s.Log("Creating a new firmware Helper instance for fixture: ", i.String())
	i.initHelper(ctx, s)

	if err := firmware.DockerClientPing(ctx); err != nil {
		s.Fatal("Failed Docker Daemon Ping After InitHelper: ", err)
	}
	s.Log("Ping Success After InitHelper")
	// If rebooting to recovery mode, verify the usb key.
	if i.value.BootMode == common.BootModeRecovery || i.value.BootMode == common.BootModeUSBDev {
		if err := i.value.Helper.RequireServo(ctx); err != nil {
			s.Fatal("Failed to connect to servod: ", err)
		}
		skipFlashUSB := false
		if skipFlashUSBStr, ok := s.Var("firmware.skipFlashUSB"); ok {
			skipFlashUSB, err = strconv.ParseBool(skipFlashUSBStr)
			if err != nil {
				s.Fatalf("Invalid value for var firmware.skipFlashUSB: got %q, want true/false", skipFlashUSBStr)
			}
		}
		cs := s.CloudStorage()
		if skipFlashUSB {
			cs = nil
		}

		if err := firmware.DockerClientPing(ctx); err != nil {
			s.Fatal("Failed Docker Daemon Ping After RequireServo: ", err)
		}
		s.Log("Ping Success After RequireServo")
		if err := i.value.Helper.SetupUSBKey(ctx, cs); err != nil {
			s.Fatal("Failed to setup USB key: ", err)
		}
	}
	return i.value
}

// Reset is called by the framework after each test (except for the last one) to do a
// light-weight reset of the environment to the original state.
func (i *impl) Reset(ctx context.Context) error {
	// Close the servo to reset pd role, watchdogs, etc.
	i.value.Helper.CloseServo(ctx)
	// Close the RPC client in case the DUT rebooted at some point, and it doesn't recover well.
	i.value.Helper.CloseRPCConnection(ctx)
	return nil
}

// PreTest is called by the framework before each test to do a light-weight set up for the test.
func (i *impl) PreTest(ctx context.Context, s *testing.FixtTestState) {
	if i.disallowSSH {
		return
	}
	// The GBB flags might prevent booting into the correct mode, so check the boot mode,
	// then save the GBB flags, then set the GBB flags, and finally reboot into the right mode.
	mode, err := i.value.Helper.Reporter.CurrentBootMode(ctx)
	if err != nil {
		s.Fatal("Failed to get current boot mode: ", err)
	}

	// If this is the first PreTest invocation, save the starting boot mode.
	// This isn't in SetUp to avoid reading CurrentBootMode twice.
	if i.origBootMode == nil {
		s.Logf("Saving boot mode %q for restoration upon completion of all tests under this fixture", mode)
		i.origBootMode = &mode
	}

	s.Log("Get current GBB flags")
	curr, err := common.GetGBBFlags(ctx, i.value.Helper.DUT)
	if err != nil {
		s.Fatal("Failed to read GBB flags: ", err)
	}

	// If this is the first PreTest invocation, save the starting GBB flags.
	// This isn't in SetUp to avoid reading GetGBBFlags twice. (It's very slow)
	if i.origGBBFlags == nil {
		i.origGBBFlags = common.CopyGBBFlags(*curr)
		// For backwards compatibility with Tauto FAFT tests, firmware.no_ec_sync=true will leave DISABLE_EC_SOFTWARE_SYNC set after the test is over. See b/194807451
		// TODO(jbettis): Consider revisiting this flag with something better.
		if common.GBBFlagsContains(i.value.GBBFlags, pb.GBBFlag_DISABLE_EC_SOFTWARE_SYNC) {
			common.GBBAddFlag(i.origGBBFlags, pb.GBBFlag_DISABLE_EC_SOFTWARE_SYNC)
		}
		testing.ContextLogf(ctx, "Saving GBB flags %+v for restoration upon completion of all tests under this fixture", i.origGBBFlags.Set)
	}

	rebootRequired := false
	if common.GBBFlagsStatesEqual(i.value.GBBFlags, *curr) {
		s.Log("GBBFlags are already proper")
	} else {
		s.Log("Setting GBB flags to ", i.value.GBBFlags.Set)
		if err := common.SetGBBFlags(ctx, i.value.Helper.DUT, i.value.GBBFlags.Set); err != nil {
			s.Fatal("SetGBBFlags failed: ", err)
		}
		if common.GBBFlagsChanged(*curr, i.value.GBBFlags, common.RebootRequiredGBBFlags()) {
			s.Log("Resetting DUT due to GBB flag change")
			rebootRequired = true
		}
	}

	if mode != i.value.BootMode {
		testing.ContextLogf(ctx, "Current boot mode is %q, rebooting to %q to satisfy fixture", mode, i.value.BootMode)
		rebootRequired = true
	}

	if rebootRequired {
		opts := []firmware.ModeSwitchOption{firmware.AssumeGBBFlagsCorrect}
		if i.value.ForcesDevMode {
			opts = append(opts, firmware.AllowGBBForce)
		}
		// Only copy the tast files if it is allowed, and also if we are in the boot mode we started in.
		if i.copyTastFiles && mode == *i.origBootMode {
			opts = append(opts, firmware.CopyTastFiles)
		}
		if err := i.rebootToMode(ctx, i.value.BootMode, opts...); err != nil {
			s.Fatalf("Failed to reboot to mode %q: %s", i.value.BootMode, err)
		}
	}
}

// PostTest is called by the framework after each test to tear down changes PreTest made.
func (i *impl) PostTest(ctx context.Context, s *testing.FixtTestState) {
	if err := i.value.Helper.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servod: ", err)
	}
	if err := i.value.Helper.EnsureDUTBooted(ctx); err != nil {
		s.Fatal("DUT is offline after test end: ", err)
	}
}

// TearDown is called by the framework to tear down the environment SetUp set up.
func (i *impl) TearDown(ctx context.Context, s *testing.FixtState) {
	defer func(ctx context.Context) {
		i.closeHelper(ctx, s)
		i.origBootMode = nil
		i.origGBBFlags = nil
	}(ctx)

	// Close the servo to reset pd role, watchdogs, etc.
	i.value.Helper.CloseServo(ctx)
	// Close the RPC client in case the DUT rebooted at some point, and it doesn't recover well.
	i.value.Helper.CloseRPCConnection(ctx)

	if err := i.value.Helper.EnsureDUTBooted(ctx); err != nil {
		s.Fatal("DUT is offline after test end: ", err)
	}

	if i.disallowSSH {
		return
	}

	rebootRequired := false
	toMode := common.BootModeUnspecified
	setGBBFlagsAfterReboot := false
	if i.origBootMode != nil {
		mode, err := i.value.Helper.Reporter.CurrentBootMode(ctx)
		if err != nil {
			s.Fatal("Failed to get boot mode: ", err)
		}
		if mode != *i.origBootMode {
			s.Logf("Restoring boot mode from %q to %q", mode, *i.origBootMode)
			rebootRequired = true
			toMode = *i.origBootMode
		}
	}

	if i.origGBBFlags != nil {
		testing.ContextLog(ctx, "Get current GBB flags")
		curr, err := common.GetGBBFlags(ctx, i.value.Helper.DUT)
		if err != nil {
			s.Fatal("Getting current GBB Flags failed: ", err)
		}

		if !common.GBBFlagsStatesEqual(*i.origGBBFlags, *curr) {
			tempGBBFlags := common.CopyGBBFlags(*i.origGBBFlags)

			// If we need to reboot the boot mode, we must have common.FAFTGBBFlags() set, but then we might have to restore the GBB flags again.
			if rebootRequired {
				common.GBBAddFlag(tempGBBFlags, common.FAFTGBBFlags()...)
				setGBBFlagsAfterReboot = !common.GBBFlagsStatesEqual(*tempGBBFlags, *i.origGBBFlags)
			}

			s.Log("Setting GBB flags to ", tempGBBFlags.Set)
			if err := common.ClearAndSetGBBFlags(ctx, i.value.Helper.DUT, *tempGBBFlags); err != nil {
				s.Fatal("Restore GBB flags failed: ", err)
			}
			if common.GBBFlagsChanged(*curr, *tempGBBFlags, common.RebootRequiredGBBFlags()) {
				s.Log("Resetting DUT due to GBB flag change")
				rebootRequired = true
			}
		}
	}

	if rebootRequired {
		opts := []firmware.ModeSwitchOption{firmware.AssumeGBBFlagsCorrect}
		if i.origGBBFlags != nil {
			if common.GBBFlagsContains(*i.origGBBFlags, pb.GBBFlag_FORCE_DEV_SWITCH_ON) {
				opts = append(opts, firmware.AllowGBBForce)
			}
		}
		if err := i.rebootToMode(ctx, toMode, opts...); err != nil {
			s.Errorf("Failed to reboot to mode %q: %s", toMode, err)
		}
		// Make sure the DUT is booted, just in case the rebootToMode failed.
		if err := i.value.Helper.EnsureDUTBooted(ctx); err != nil {
			s.Fatal("DUT is offline after test end: ", err)
		}
		if setGBBFlagsAfterReboot {
			s.Log("Setting GBB flags to ", i.origGBBFlags.Set)
			if err := common.ClearAndSetGBBFlags(ctx, i.value.Helper.DUT, *i.origGBBFlags); err != nil {
				s.Fatal("Restore GBB flags failed: ", err)
			}
		}
	}
}

// String identifies this fixture.
func (i *impl) String() string {
	name := string(i.value.BootMode)
	if i.value.ForcesDevMode {
		name += "-gbb"
	}
	return name
}

// initHelper ensures that the impl has a working Helper instance.
func (i *impl) initHelper(ctx context.Context, s *testing.FixtState) {
	if i.value.Helper == nil {
		servoSpec, _ := s.Var("servo")

		if i.disallowSSH {
			i.value.Helper = firmware.NewHelper(nil, nil, s.DataPath(firmware.ConfigFile), servoSpec, "", "", "", "")
			i.value.Helper.DisallowServices()
		} else {
			dutHostname, _ := s.Var("dutHostname")
			if dutHostname == "" {
				host, _, err := net.SplitHostPort(s.DUT().HostName())
				if err != nil {
					testing.ContextLogf(ctx, "Failed to extract DUT hostname from %q, use --var=dutHostname to set", s.DUT().HostName())
				}
				dutHostname = host
			}
			powerunitHostname, _ := s.Var("powerunitHostname")
			powerunitOutlet, _ := s.Var("powerunitOutlet")
			hydraHostname, _ := s.Var("hydraHostname")
			i.value.Helper = firmware.NewHelper(s.DUT(), s.RPCHint(), s.DataPath(firmware.ConfigFile), servoSpec, dutHostname, powerunitHostname, powerunitOutlet, hydraHostname)
			if !i.copyTastFiles {
				i.value.Helper.DisallowServices()
			}
		}
	}
}

// closeHelper closes and nils any existing Helper instance.
func (i *impl) closeHelper(ctx context.Context, s *testing.FixtState) {
	if i.value.Helper == nil {
		return
	}
	if err := i.value.Helper.Close(ctx); err != nil {
		s.Log("Failed to close helper: ", err)
	}
	i.value.Helper = nil
}

// rebootToMode reboots to the specified mode using the ModeSwitcher, it assumes the helper is present.
func (i *impl) rebootToMode(ctx context.Context, mode common.BootMode, opts ...firmware.ModeSwitchOption) error {
	ms, err := firmware.NewModeSwitcher(ctx, i.value.Helper)
	if err != nil {
		return errors.Wrap(err, "failed to create mode switcher")
	}
	if mode == common.BootModeUnspecified {
		if err := ms.ModeAwareReboot(ctx, firmware.WarmReset); err != nil {
			return errors.Wrap(err, "failed to warm reboot")
		}
		return nil
	}
	if err := ms.RebootToMode(ctx, mode, opts...); err != nil {
		return errors.Wrapf(err, "failed to reboot to mode %q", mode)
	}
	return nil
}
