// Copyright 2019 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"bytes"
	"context"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/common"
	"chromiumos/tast/local/crosconfig"
	"chromiumos/tast/local/firmware"
	"chromiumos/tast/local/input"
	fwpb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			fwpb.RegisterUtilsServiceServer(srv, &UtilsService{
				s:            s,
				sharedObject: common.SharedObjectsForServiceSingleton,
			})
		},
	})
}

// UtilsService implements tast.cros.firmware.UtilsService.
type UtilsService struct {
	s            *testing.ServiceState
	cr           *chrome.Chrome
	sharedObject *common.SharedObjectsForService
}

// BlockingSync syncs the root device and internal device.
func (*UtilsService) BlockingSync(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	// The double calls to sync fakes a blocking call
	// since the first call returns before the flush is complete,
	// but the second will wait for the first to finish.
	for i := 0; i < 2; i++ {
		if err := testexec.CommandContext(ctx, "sync").Run(testexec.DumpLogOnError); err != nil {
			return nil, errors.Wrapf(err, "sending sync command #%d to DUT", i+1)
		}
	}

	// Find the root device.
	rootDevice, err := firmware.RootDevice(ctx)
	if err != nil {
		return nil, err
	}
	devices := []string{rootDevice}

	// If booted from removable media, sync the internal device too.
	isRemovable, err := firmware.BootDeviceRemovable(ctx)
	if err != nil {
		return nil, err
	}
	if isRemovable {
		internalDevice, err := firmware.InternalDevice(ctx)
		if err != nil {
			return nil, err
		}
		devices = append(devices, internalDevice)
	}

	// sync only sends SYNCHRONIZE_CACHE but doesn't check the status.
	// This function will perform a device-specific sync command.
	for _, device := range devices {
		if strings.Contains(device, "mmcblk") {
			// For mmc devices, use `mmc status get` command to send an
			// empty command to wait for the disk to be available again.
			if err := testexec.CommandContext(ctx, "mmc", "status", "get", device).Run(testexec.DumpLogOnError); err != nil {
				return nil, errors.Wrapf(err, "sending mmc command to device %s", device)
			}
		} else if strings.Contains(device, "nvme") {
			// For NVMe devices, use `nvme flush` command to commit data
			// and metadata to non-volatile media.
			// Get a list of NVMe namespaces, and flush them individually.
			// The output is assumed to be in the following format:
			// [ 0]:0x1
			// [ 1]:0x2
			namespaces, err := testexec.CommandContext(ctx, "nvme", "list-ns", device).Output(testexec.DumpLogOnError)
			if err != nil {
				return nil, errors.Wrapf(err, "listing namespaces for device %s", device)
			}
			if len(namespaces) == 0 {
				return nil, errors.Errorf("Listing namespaces for device %s returned no output", device)
			}
			for _, namespace := range strings.Split(strings.TrimSpace(string(namespaces)), "\n") {
				ns := strings.Split(namespace, ":")[1]
				if err := testexec.CommandContext(ctx, "nvme", "flush", device, "-n", ns).Run(testexec.DumpLogOnError); err != nil {
					return nil, errors.Wrapf(err, "flushing namespace %s on device %s", ns, device)
				}
			}
		} else {
			// For other devices, hdparm sends TUR to check if
			// a device is ready for transfer operation.
			if err := testexec.CommandContext(ctx, "hdparm", "-f", device).Run(testexec.DumpLogOnError); err != nil {
				return nil, errors.Wrapf(err, "sending hdparm command to device %s", device)
			}
		}
	}
	return &empty.Empty{}, nil
}

// ReadServoKeyboard reads from the servo's keyboard emulator.
func (us *UtilsService) ReadServoKeyboard(ctx context.Context, req *fwpb.ReadServoKeyboardRequest) (*fwpb.ReadServoKeyboardResponse, error) {
	// The servo's keyboard emulator device node has a symlink, captured here as a constant,
	// that will always link to the actual node.  When the node that the symlink links to
	// gets assigned different values, such assignments are transparent, since we use the
	// symlink.
	const node = "/dev/input/by-id/usb-Google_Servo_LUFA_Keyboard_Emulator-event-kbd"
	// TODO(kmshelton): Migrate to using a library (i.e. invoking functions from golang-evdev
	// instead of invoking a binary on the test image), if the usecase of using evdev bindings
	// is considered strong enough to deal with the drawbacks that come with enabling cgo in
	// tast (b:187786098).
	ctx, cancel := context.WithTimeout(ctx, time.Duration(req.Duration)*time.Second)
	defer cancel()
	cmd := testexec.CommandContext(ctx, "evtest", "--grab", node)
	stdout := &bytes.Buffer{}
	cmd.Stdout = stdout
	err := cmd.Start()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read keyboard")
	}
	if err = cmd.Wait(); err == nil {
		return nil, errors.New("evtest unexpectedly did not time out")
	} else if !errors.Is(err, context.DeadlineExceeded) {
		return nil, errors.Wrap(err, "evtest exited unexpectedly")
	}
	// An occurrence of "value 0" in evtest output corresponds to a press of a key, whereas "value 1" is a release.
	re := regexp.MustCompile(`\(KEY_([A-Z0-9_]+)\), value 0`)
	matches := re.FindAllSubmatch(stdout.Bytes(), -1)
	var keys []string
	for _, match := range matches {
		keys = append(keys, string(match[1]))
	}
	us.s.Log("Detected keys on the DUT: ", keys)
	return &fwpb.ReadServoKeyboardResponse{Keys: keys}, nil
}

func (us *UtilsService) FindPhysicalKeyboard(ctx context.Context, req *empty.Empty) (*fwpb.InputDevicePath, error) {
	foundKB, path, err := input.FindPhysicalKeyboard(ctx)
	if err != nil {
		return nil, err
	} else if !foundKB {
		return nil, errors.New("no physical keyboard found")
	} else {
		return &fwpb.InputDevicePath{Path: path}, nil
	}
}

func (us *UtilsService) FindPowerKeyDevice(ctx context.Context, req *empty.Empty) (*fwpb.InputDevicePath, error) {
	foundPowerKey, path, err := input.FindPowerKeyDevice(ctx)
	if err != nil {
		return nil, err
	} else if !foundPowerKey {
		return nil, errors.New("no input device for power key found")
	} else {
		return &fwpb.InputDevicePath{Path: path}, nil
	}
}

// NewChrome starts a new Chrome session and logs in as a test user.
func (us *UtilsService) NewChrome(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if us.cr != nil {
		return nil, errors.New("Chrome already available")
	}

	cr, err := chrome.New(ctx)
	if err != nil {
		return nil, err
	}
	us.cr = cr
	return &empty.Empty{}, nil
}

// CloseChrome closes a Chrome session and cleans up the resources obtained by NewChrome.
func (us *UtilsService) CloseChrome(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if us.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	err := us.cr.Close(ctx)
	us.cr = nil
	return &empty.Empty{}, err
}

// ReuseChrome reuses the existing Chrome session if there's already one.
func (us *UtilsService) ReuseChrome(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if us.cr != nil {
		testing.ContextLog(ctx, "Chrome already available")
		return &empty.Empty{}, nil
	}

	// First, look up the shared Chrome instance set by CheckVirtualKeyboarService (or other services).
	// Otherwise, reuse the one created by NewChrome in this service with the same options.
	us.sharedObject.ChromeMutex.Lock()
	defer us.sharedObject.ChromeMutex.Unlock()
	if us.sharedObject.Chrome != nil {
		us.cr = us.sharedObject.Chrome
	} else {
		cr, err := chrome.New(ctx, chrome.TryReuseSession())
		if err != nil {
			return nil, err
		}
		us.cr = cr
	}
	return &empty.Empty{}, nil
}

// EvalTabletMode evaluates tablet mode status.
func (us *UtilsService) EvalTabletMode(ctx context.Context, req *empty.Empty) (*fwpb.EvalTabletModeResponse, error) {
	if us.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	tconn, err := us.cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "creating test API connection failed")
	}
	// Check if tablet mode is enabled on DUT.
	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tablet mode enabled status")
	}
	return &fwpb.EvalTabletModeResponse{TabletModeEnabled: tabletModeEnabled}, nil
}

// FindSingleNode finds the specific UI node based on the passed in element.
func (us *UtilsService) FindSingleNode(ctx context.Context, req *fwpb.NodeElement) (*empty.Empty, error) {
	if us.cr == nil {
		return nil, errors.New("missing chrome instance")
	}

	tconn, err := us.cr.TestAPIConn(ctx)
	if err != nil {
		return nil, err
	}

	uiauto := uiauto.New(tconn)
	uiNode := nodewith.Name(req.Name).First()

	if err := uiauto.WithTimeout(10 * time.Second).WaitUntilExists(uiNode)(ctx); err != nil {
		if err := saveLogsOnError(ctx, us, func() bool { return true }); err != nil {
			return nil, errors.Wrapf(err, "could not save logs when node %q not found", req.Name)
		}
		return nil, errors.Wrapf(err, "could not find node: %s", req.Name)
	}

	return &empty.Empty{}, nil
}

func saveLogsOnError(ctx context.Context, us *UtilsService, hasError func() bool) error {
	tconn, err := us.cr.TestAPIConn(ctx)
	if err != nil {
		return err
	}

	outDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return errors.New("could not get the output directory")
	}
	faillog.DumpUITreeOnError(ctx, filepath.Join(outDir, "BaseECUpdate"), hasError, tconn)
	faillog.SaveScreenshotOnError(ctx, us.cr, filepath.Join(outDir, "BaseECUpdate"), hasError)
	return nil
}

// GetDetachableBaseValue retrieves the values of a few detachable-base attributes,
// such as product-id, usb-path, and vendor-id. The values are saved and returned
// in a list.
func (us *UtilsService) GetDetachableBaseValue(ctx context.Context, req *empty.Empty) (*fwpb.CrosConfigResponse, error) {
	paramsSlice := []string{"product-id", "vendor-id", "usb-path"}

	crosCfgRes := fwpb.CrosConfigResponse{}

	for _, v := range paramsSlice {
		value, err := crosconfig.Get(ctx, "/detachable-base", v)
		if err != nil {
			return nil, err
		}
		if value == "" {
			return nil, errors.Errorf("%s is empty", v)
		}
		switch v {
		case "product-id":
			crosCfgRes.ProductId = value
		case "vendor-id":
			crosCfgRes.VendorId = value
		case "usb-path":
			crosCfgRes.UsbPath = value
		}
	}
	return &crosCfgRes, nil
}

// PerformSpeedometerTest runs the speedometer test on one external website, and returns the result value.
func (us *UtilsService) PerformSpeedometerTest(ctx context.Context, req *empty.Empty) (*fwpb.SpeedometerResponse, error) {
	// Verify we have logged in.
	if us.cr == nil {
		return nil, errors.New("Chrome not available")
	}

	// Open the Speedometer website.
	conn, err := us.cr.NewConn(ctx, "https://browserbench.org/Speedometer2.0/")
	if err != nil {
		return nil, errors.Wrap(err, "failed to open Speedometer website")
	}
	defer conn.Close()

	// Connect to Test API to use it with the UI library.
	tconn, err := us.cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create test API connection")
	}

	// Find and click on the 'Start Test' button.
	uia := uiauto.New(tconn)
	startButton := nodewith.Name("Start Test").Role(role.Button).Onscreen()
	if err := uiauto.Combine("Click Start Test",
		uia.WaitUntilExists(startButton),
		uia.LeftClick(startButton),
	)(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to find and click the start button")
	}

	// Wait for the result to appear.
	title := nodewith.Name("Runs / Minute").Role(role.Heading).Onscreen()
	if err := uia.WithTimeout(10 * time.Minute).WaitUntilExists(title)(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to find the title for speedometer test result value")
	}

	// Get the result from speedometer test.
	result := nodewith.NameRegex(regexp.MustCompile(`^[0-9]+[.]?[0-9]*$`)).Role(role.StaticText).First()
	resultInfo, err := uia.Info(ctx, result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the result value")
	}

	return &fwpb.SpeedometerResponse{Result: resultInfo.Name}, err
}
