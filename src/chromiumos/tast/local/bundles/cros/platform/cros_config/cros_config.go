package cros_config

import (
	"encoding/json"
	"io/ioutil"
	"strings"

	"chromiumos/tast/testing"

	"github.com/google/go-cmp/cmp"
	"gopkg.in/yaml.v2"
)

// TODO: Replace the TastDir constant with "/usr/share/chromeos-config/tast/" once the eclass is updated to
// write the YAML and golden DB to that directory. Using /tmp/tast is great for testing, the
// /usr/share/chromeos-config is on a read-only filesystem.
const TastDir = "/tmp/tast/"

// Command struct to capture the binary to run with all of the arguments.
type Command struct {
	Binary string
	Args   []string
}

// Command function to build the lookup key used in the golden database.
func (c Command) Key() string {
	return c.Binary + "_" + strings.Join(c.Args, "_")
}

// Main YAML stuct used to reaad in all of the common and device specific commands.
type T struct {
	Devices []struct {
		DeviceName string   `yaml:"device-name"`
		Mosys      []string `yaml:"mosys"`
		CrosConfig []string `yaml:"cros_config"`
	} `yaml:"devices"`
}

// JSON struct for reading and writing the golden database file.
type GoldenDBFile struct {
	DeviceName string      `json:"device_name"`
	Records    []GoldenRec `json:"records"`
}

// JSON struct for each command and its ouput.
type GoldenRec struct {
	CommandKey string `json:"command_key"`
	Value      string `json:"value"`
}

// Compare the existing golden database to the newly built output structure.
func CompareGoldenOutput(output GoldenDBFile, goldenFilename string, s *testing.State) (bool, []string) {
	s.Logf("Trying to read Golden DB File: %q", goldenFilename)
	bytes, err := ioutil.ReadFile(goldenFilename)
	if err != nil {
		s.Error("Failed to read Golden DB File: ", err)
	}

	var golden GoldenDBFile
	json.Unmarshal(bytes, &golden)

	// Compare the two structures and produce diffs if doesn't match.
	var diffs []string
	var eq = cmp.Equal(output, golden)
	s.Logf("New output struct matches existing golden struct: %t", eq)
	if !eq {
		// Split the single string into all of the differences and add to output array.
		diffStr := cmp.Diff(output, golden)
		d := strings.Split(diffStr, "{cros_config.GoldenDBFile}.")
		for _, diff := range d {
			if len(diff) <= 0 {
				continue
			}
			s.Errorf("diff: %q", diff)
			diffs = append(diffs, diff)
		}
	}
	return eq, diffs
}

func DetermineDevicesToProcess(deviceNameFilename string, s *testing.State) ([]string, string) {
	// Determine all of the device's configs to process.
	var deviceName string = "Unknown"
	var devicesToProcess []string
	devicesToProcess = append(devicesToProcess, "all")

	// Read the device name from the tast tests. This name will drive
	// any device specific commands, monitoring and golden DB naming.
	bytes, err := ioutil.ReadFile(deviceNameFilename)
	if err != nil {
		s.Error("Failed to read device_name.txt file: ", err)
	} else {
		deviceName = strings.Trim(string(bytes), "\n")
		s.Logf("Adding device: %q", deviceName)
		devicesToProcess = append(devicesToProcess, deviceName)
	}

	return devicesToProcess, deviceName
}

func BuildCommands(b []byte, devicesToProcess []string, s *testing.State) []Command {
	var commands []Command

	t := T{}
	err := yaml.Unmarshal(b, &t)
	if err != nil {
		s.Fatal("Failed to unmarshall yaml to struct: ", err)
	}

	for _, v := range t.Devices {
		if Contains(devicesToProcess, v.DeviceName) {
			for _, args := range v.Mosys {
				commands = append(commands, BuildCommand("mosys", args))
			}
			for _, args := range v.CrosConfig {
				commands = append(commands, BuildCommand("cros_config", args))
			}
		}
	}

	return commands
}

func BuildCommand(binary string, line string) Command {
	arr := strings.Split(strings.Trim(line, "\n"), " ")
	return Command{Binary: binary, Args: arr}
}

func Contains(a []string, x string) bool {
	for _, n := range a {
		if x == n {
			return true
		}
	}
	return false
}
