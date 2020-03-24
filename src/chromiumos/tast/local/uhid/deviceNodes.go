package uhid

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// device nodes refer to the /dev/input/event and /dev/hidraw nodes.
// these are obtained from /sys/bus/hid/devices which contains hid
// device information.
func getDeviceNodes(d *UHIDDevice) error {
	devicePath, err := devicePath(d.infoString())
	if err != nil {
		return err
	}
	d.HidrawNodes, err = getHidrawNodes(devicePath)
	if err != nil {
		return err
	}
	d.EventNodes, err = getEventNodes(devicePath)
	return err
}

// An example of a possible device path is
// /sys/bus/hid/devices/0003:046D:C31C.0018 where 0003 is the bus,
// 046D is the vendor id and C31C is the product id. 0018 is a unique
// number given in case multiple devices exist with the same bus and
// ids. In the case of this library we choose to take the path of the
// most recently created device. That is, the one with the highest
// unique number.
func devicePath(infoString string) (string, error) {
	devicePath := "/sys/bus/hid/devices/" + infoString + "."
	out, err := exec.Command("sh",
		"-c",
		fmt.Sprintf("find %s -maxdepth 1", devicePath+"*")).Output()
	if err != nil {
		return "", err
	}
	if len(out) == 0 {
		return "",
			errors.New("No device was created corresponding to this information")
	}
	ids, err := ids(paths(out))
	if err != nil {
		return "", err
	}
	id := max(ids)
	return devicePath + fmt.Sprintf("%04X", id), nil
}

func getHidrawNodes(path string) ([]string, error) {
	out, err := exec.Command("sh",
		"-c",
		fmt.Sprintf("ls %s/hidraw/ | grep hidraw*", path)).Output()
	if err != nil {
		return nil, err
	}
	hidrawPaths := hidrawDevPaths(strings.Split(string(out), "\n"))
	return hidrawPaths[:len(hidrawPaths)-1], nil
}

func getEventNodes(path string) ([]string, error) {
	out, err := exec.Command("sh",
		"-c",
		fmt.Sprintf("ls %s/input/input* | grep ^event*", path)).Output()
	if err != nil {
		return nil, err
	}
	eventPaths := inputDevPaths(strings.Split(string(out), "\n"))
	return eventPaths[:len(eventPaths)-1], nil
}

func (d *UHIDDevice) infoString() string {
	return fmt.Sprintf("%04X:%04X:%04X", d.Data.Bus,
		d.Data.VendorId,
		d.Data.ProductId)
}

func paths(b []byte) []string {
	paths := strings.Split(string(b), "\n")
	return paths[:len(paths)-1]
}

func ids(paths []string) ([]int64, error) {
	ids := make([]int64, 0)
	for _, v := range paths {
		nextId, err := getId(v)
		if err != nil {
			return ids, err
		}
		ids = append(ids, nextId)
	}
	return ids, nil
}

func getId(path string) (int64, error) {
	startOfId := strings.Index(path, ".") + 1
	return strconv.ParseInt(path[startOfId:], 16, 16)
}

func max(a []int64) int64 {
	if len(a) == 0 {
		return -1
	}
	max := a[0]
	for _, v := range a {
		if max < v {
			max = v
		}
	}
	return max
}

func hidrawDevPaths(incompletePaths []string) []string {
	return prependToStringArray(incompletePaths, "/dev/")
}

func inputDevPaths(incompletePaths []string) []string {
	return prependToStringArray(incompletePaths, "/dev/input/")
}

func prependToStringArray(a []string, prepended string) []string {
	ret := make([]string, 0)
	for _, v := range a {
		ret = append(ret, prepended+v)
	}
	return ret
}
