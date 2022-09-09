// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"golang.org/x/sys/unix"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

type params struct {
	frames int
	effect string
	width  int
	height int
	image  string
}

type metric struct {
	Name  string  `json:"name"`
	Unit  string  `json:"unit"`
	Value float64 `json:"value"`
}

type output struct {
	Metrics []metric `json:"metrics"`
}

func init() {
	testing.AddTest(&testing.Test{
		Func: EffectsBenchmark,
		Desc: "Runs the Effects benchmark",
		Contacts: []string{
			"jakebarnes@google.com",
			"chromeos-platform-ml@google.com",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"camera_feature_effects"},
		Fixture:      "powerSetUp",
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Name:      "none_1080p",
				ExtraData: []string{"wfh_1080p.nv12"},
				Val: params{
					frames: 1000,
					effect: "none",
					width:  1920,
					height: 1080,
					image:  "wfh_1080p.nv12",
				},
			},
			{
				Name:      "none_720p",
				ExtraData: []string{"wfh_720p.nv12"},
				Val: params{
					frames: 1000,
					effect: "none",
					width:  1280,
					height: 720,
					image:  "wfh_720p.nv12",
				},
			},
			{
				Name:      "none_360p",
				ExtraData: []string{"wfh_360p.nv12"},
				Val: params{
					frames: 1000,
					effect: "none",
					width:  640,
					height: 360,
					image:  "wfh_360p.nv12",
				},
			},
			{
				Name:      "blur_1080p",
				ExtraData: []string{"wfh_1080p.nv12"},
				Val: params{
					frames: 1000,
					effect: "blur",
					width:  1920,
					height: 1080,
					image:  "wfh_1080p.nv12",
				},
			},
			{
				Name:      "blur_720p",
				ExtraData: []string{"wfh_720p.nv12"},
				Val: params{
					frames: 1000,
					effect: "blur",
					width:  1280,
					height: 720,
					image:  "wfh_720p.nv12",
				},
			},
			{
				Name:      "blur_360p",
				ExtraData: []string{"wfh_360p.nv12"},
				Val: params{
					frames: 1000,
					effect: "blur",
					width:  640,
					height: 360,
					image:  "wfh_360p.nv12",
				},
			},
			{
				Name:      "relight_1080p",
				ExtraData: []string{"wfh_1080p.nv12"},
				Val: params{
					frames: 1000,
					effect: "relight",
					width:  1920,
					height: 1080,
					image:  "wfh_1080p.nv12",
				},
			},
			{
				Name:      "relight_720p",
				ExtraData: []string{"wfh_720p.nv12"},
				Val: params{
					frames: 1000,
					effect: "relight",
					width:  1280,
					height: 720,
					image:  "wfh_720p.nv12",
				},
			},
			{
				Name:      "relight_360p",
				ExtraData: []string{"wfh_360p.nv12"},
				Val: params{
					frames: 1000,
					effect: "relight",
					width:  640,
					height: 360,
					image:  "wfh_360p.nv12",
				},
			},
			{
				Name:      "replace_1080p",
				ExtraData: []string{"wfh_1080p.nv12"},
				Val: params{
					frames: 1000,
					effect: "replace",
					width:  1920,
					height: 1080,
					image:  "wfh_1080p.nv12",
				},
			},
			{
				Name:      "replace_720p",
				ExtraData: []string{"wfh_720p.nv12"},
				Val: params{
					frames: 1000,
					effect: "replace",
					width:  1280,
					height: 720,
					image:  "wfh_720p.nv12",
				},
			},
			{
				Name:      "replace_360p",
				ExtraData: []string{"wfh_360p.nv12"},
				Val: params{
					frames: 1000,
					effect: "replace",
					width:  640,
					height: 360,
					image:  "wfh_360p.nv12",
				},
			},
		},
	})
}

// timeToDuration converts the difference in two unix.Timeval objects to a time.Duration object.
func timeToDuration(startTime, endTime unix.Timeval) time.Duration {
	return time.Duration(endTime.Sec)*time.Second +
		time.Duration(endTime.Usec)*time.Microsecond -
		time.Duration(startTime.Sec)*time.Second -
		time.Duration(startTime.Usec)*time.Microsecond
}

func EffectsBenchmark(ctx context.Context, s *testing.State) {
	p, ok := s.Param().(params)
	if !ok {
		s.Fatal("Failed to convert test params")
	}

	// Prepare command, capturing output to log file
	outputFilename := filepath.Join(s.OutDir(), "output.json")
	logFile, err := os.Create(
		filepath.Join(s.OutDir(), "effects_benchmark.log"))
	if err != nil {
		s.Fatal("Failed to create log file: ", logFile)
	}
	defer logFile.Close()
	cmd := testexec.CommandContext(ctx,
		"effects_benchmark",
		"--frames="+strconv.Itoa(p.frames),
		"--effect="+p.effect,
		"--image="+s.DataPath(p.image),
		"--width="+strconv.Itoa(p.width),
		"--height="+strconv.Itoa(p.height),
		"--output_path="+outputFilename)
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	raplEnergyBefore, err := power.NewRAPLSnapshot()
	if err != nil {
		testing.ContextLog(ctx, "RAPL Energy status is not available for this board")
	}

	var rusageStart, rusageEnd unix.Rusage
	unix.Getrusage(unix.RUSAGE_CHILDREN, &rusageStart)

	if err := cmd.Run(); err != nil {
		s.Fatal("Failed to run benchmark: ", err)
	}

	unix.Getrusage(unix.RUSAGE_CHILDREN, &rusageEnd)

	var energyDiff *power.RAPLValues
	if raplEnergyBefore != nil {
		energyDiff, err = raplEnergyBefore.DiffWithCurrentRAPL()
		if err != nil {
			s.Fatal("Failed to get RAPL power usage: ", err)
		}
	}

	// Populate metrics
	values := perf.NewValues()
	values.Set(perf.Metric{
		Name: "rusage_cpu_time_user",
		Unit: "s",
	}, float64(timeToDuration(rusageStart.Utime, rusageEnd.Utime).Seconds()))
	values.Set(perf.Metric{
		Name: "rusage_cpu_time_system",
		Unit: "s",
	}, float64(timeToDuration(rusageStart.Stime, rusageEnd.Stime).Seconds()))
	values.Set(perf.Metric{
		Name: "rusage_maxrss",
		Unit: "bytes",
	}, float64(rusageEnd.Maxrss*1024))
	if energyDiff != nil {
		raplPower := energyDiff.Total()
		values.Set(perf.Metric{
			Name: "rapl_energy_total",
			Unit: "J",
		}, raplPower)
		values.Set(perf.Metric{
			Name: "rapl_energy_per_frame",
			Unit: "J",
		}, raplPower/float64(p.frames))
	}

	outputJSON, err := ioutil.ReadFile(outputFilename)
	if err != nil {
		s.Fatal("Failed to read output file: ", err)
	}
	var output output
	if err := json.Unmarshal(outputJSON, &output); err != nil {
		s.Fatal("Failed to parse output file: ", err)
	}
	for _, metric := range output.Metrics {
		values.Set(perf.Metric{
			Name: metric.Name,
			Unit: metric.Unit,
		}, metric.Value)
	}

	if err := values.Save(s.OutDir()); err != nil {
		s.Error("Cannot save perf data: ", err)
	}
}
