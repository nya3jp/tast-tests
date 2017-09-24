package power

import (
	"fmt"
	"os/exec"
	"strconv"
)

type BacklightType int

const (
	DisplayBacklight BacklightType = iota
	KeyboardBacklight
)

func runBacklightTool(bt backlightType, args ...string) ([]byte, error) {
	if bt == KeyboardBacklight {
		args = append([]string{"--keyboard"}, args...)
	}
	return exec.Command("backlight_tool", args...).Output()
}

// GetBrightness returns the specified backlight's current brightness, as
// a hardware-specific level between 0 and the maximum returned by
// GetMaxBrightness, inclusive.
func GetBrightness(bt backlightType) (int64, error) {
	b, err := runBacklightTool(bt, "--get_brightness")
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(string(b), 10, 64)
}

// GetMaxBrightness returns the specified backlight's maximum brightness, as
// a hardware-specific level.
func GetMaxBrightness(bt backlightType) (int64, error) {
	b, err := runBacklightTool(bt, "--get_max_brightness")
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(string(b), 10, 64)
}

// GetInitialBrightness returns the initial brightness level that powerd would
// use for the specified backlight.
func GetInitialBrightness(bt backlightType) (int64, error) {
	b, err := runBacklightTool(bt, "--get_initial_brightness")
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(string(b), 10, 64)
}

// SetBrightness sets the specified backlight's brightness level.
func SetBrightness(bt backlightType, level int64) error {
	_, err := runBacklightTool(bt, fmt.Sprintf("--set_brightness=%d", level))
	return err
}
