package uhid

import (
	"chromiumos/tast/testing"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	devicesDirectory = "/sys/bus/hid/devices/"
	hidRawDir        = "hidraw"
)

// deviceNodes returns the nodes corresponding to this device.
// these nodes refer to the /dev/input/event and /dev/hidraw nodes.
// these are obtained from /sys/bus/hid/devices which contains hid
// device information.
func deviceNodes(ctx context.Context, d *UHIDDevice) error {
	devicePath, err := devicePath(d.infoString())
	if err != nil {
		return err
	}
	if d.hidrawNodes, err = hidRawNodes(ctx, devicePath); err != nil {
		return err
	}
	d.eventNodes, err = eventNodes(devicePath)
	return err
}

// devicePath return the path corresponding to this device that
// exists in /sys/bus/hid/devices/.
// An example of a possible device path is
// /sys/bus/hid/devices/0003:046D:C31C.0018 where 0003 is the bus,
// 046D is the vendor id and C31C is the product id. 0018 is a unique
// number given in case multiple devices exist with the same bus and
// ids. In the case of this library we choose to take the path of the
// most recently created device. That is, the one with the highest
// unique number.
func devicePath(infoString string) (string, error) {
	files, err := ioutil.ReadDir(devicesDirectory)
	if err != nil {
		return "", err
	}
	devicePath := ""
	deviceID := -1
	for _, f := range files {
		var currentID int
		if currentID, err = id(f.Name()); err != nil {
			return "", err
		}
		if currentID > deviceID && strings.HasPrefix(f.Name(), infoString) {
			deviceID = currentID
			devicePath = f.Name()
		}
	}
	if devicePath == "" {
		return "", fmt.Errorf("device %p hasn't been created", infoString)
	}
	return devicesDirectory + devicePath + "/", nil
}

// hidRawNodes returns the hidraw nodes that exist under
// <path>/hidraw.  Because the hidraw directory takes some time to be
// createad we poll for it.
func hidRawNodes(ctx context.Context, path string) ([]string, error) {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		directories, err := ioutil.ReadDir(path)
		if err != nil {
			return err
		}
		for _, d := range directories {
			if d.Name() == hidRawDir {
				return nil
			}
		}
		return errors.New("hidraw directory was not created")
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return nil, err
	}
	path = path + hidRawDir + "/"
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}
	return hidRawPaths(files), nil
}

// eventNodes returns the event nodes under <path>/input/input*.
// A device can have multiple directories like this. For example,
// a dualshock 3 controller will have <path>/input/input<i> and
// <path>/input/input<i+1> which represent the controller and
// its motion sensors.
func eventNodes(path string) ([]string, error) {
	eventNodes := make([]string, 0)
	directories, err := ioutil.ReadDir(path + "input/")
	if err != nil {
		return nil, err
	}
	for _, d := range directories {
		if strings.HasPrefix(d.Name(), "input") {
			eventNode, err := eventNode(path + "input/" + d.Name() + "/")
			if err != nil {
				return nil, err
			}
			eventNodes = append(eventNodes, eventNode)
		}
	}
	if len(eventNodes) == 0 {
		return nil, errors.New("the created device has no event nodes")
	}
	return eventNodes, nil
}

// infoString returns a string of the form
// <d.Data.Bus>:<d.Data.VendorID>:<d.Data.ProductID>. Information
// regarding this device will be found under
// /sys/bus/hid/devices/<infoString>.<ID> where ID is a unique ID for
// each device the kernel recognizes.
func (d *UHIDDevice) infoString() string {
	return fmt.Sprintf("%04X:%04X:%04X", d.Data.Bus, d.Data.VendorID, d.Data.ProductID)
}

// ID returns the unique ID belonging to the device represented by the
// directory in path.
func id(path string) (int, error) {
	id, err := strconv.ParseInt(filepath.Ext(path)[1:], 16, 0)
	if err != nil {
		return -1, errors.New("the given path is not a sysfs device path")
	}
	return int(id), nil
}

// hidrawPaths returns the file names of the files in files prepended
// with "/dev/" which creates their absolute path. It filters out of
// files the none hidraw files
func hidRawPaths(files []os.FileInfo) []string {
	paths := make([]string, 0)
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "hidraw") {
			paths = append(paths, "/dev/"+f.Name())
		}
	}
	return paths
}

// eventNode gets the event* node that exists inside path and prepends
// to it "/dev/input/" to create its absolute path.
func eventNode(path string) (string, error) {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return "", err
	}
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "event") {
			return "dev/input/" + f.Name(), nil
		}
	}
	return "", nil
}
