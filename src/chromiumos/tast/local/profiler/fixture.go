
package profiler

import (
	"context"
	"time"
	"strings"

	"chromiumos/tast/testing"
	"chromiumos/tast/errors"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:   "profilerRunning",
		Desc:   "Started profilers specified by profiler.AccessVars.mode variable",
		Contacts: []string{"jacobraz@google.com"},
		Impl: NewProfilerFixture("sched"),
		SetUpTimeout:100 * time.Second,
		ResetTimeout: 5 * time.Second,
		TearDownTimeout: 100 * time.Second,
	})
}

type profilerFixture struct{
	modes string 
	outDir string
	runningProfs *RunningProf
}

func NewProfilerFixture(mode string) *profilerFixture {
	return &profilerFixture{modes: mode}
}

func (f *profilerFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	//TODO handle aarch64 devices that cant run perf
	args := strings.Fields(f.modes)
	profs := make([]Profiler, 0)
	var stat PerfStatOutput
	var sched PerfSchedOutput

	//ctx has been canceled before getting to TearDown so need to make a new one that persists
	for _, arg := range args {
		switch arg {
		case "stat":
			profs = append(profs, Perf(PerfStatOpts(&stat, 0)))
		case "sched":
			profs = append(profs, Perf(PerfSchedOpts(&sched, "")))
		case "record":
			profs = append(profs, Perf(PerfRecordOpts()))
		case "statrecord":
			profs = append(profs, Perf(PerfStatRecordOpts()))
		case "none":
			return nil
		default:
			//ERROR OUT unknown profiler arg

		}
	}
	f.outDir = s.OutDir()
	// This call to start with ctx means these profilers will stop running when ctx is cancelled
	// which appears to be happening before the call to End in TearDown
	rp, err := Start(ctx, s.OutDir(), profs...)
	if err != nil {
		s.Fatal("Failure in starting the profiler: ", err)
	}
	f.runningProfs = rp
	return nil
}

func (f *profilerFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	// THis call to end fails as our running profilers have already stopped running
	if err := f.runningProfs.End(ctx); err != nil {
		s.Error("Failure in ending the profiler: ", err)
	} else {
		//TODO append metrics to perfValue
	}
}

func (f *profilerFixture) Reset(ctx context.Context)  error {
	//TODO implement reset behavior (ie tearDown then SetUp
	if err := f.runningProfs.End(ctx); err != nil {
		return errors.Wrap(err, "failure in ending the profiler")
	} else {
		//TODO append metrics again
		args := strings.Fields(f.modes)
		profs := make([]Profiler, 0)

		var stat PerfStatOutput
		var sched PerfSchedOutput


		for _, arg := range args {
			switch arg {
			case "stat":
				profs = append(profs, Perf(PerfStatOpts(&stat, 0)))
			case "sched":
				profs = append(profs, Perf(PerfSchedOpts(&sched, "")))
			case "record":
				profs = append(profs, Perf(PerfRecordOpts()))
			case "statrecord":
				profs = append(profs, Perf(PerfStatRecordOpts()))
			case "none":
				return nil
			default:
				//ERROR OUT unknown profiler arg

			}

		}
		rp, err := Start(ctx, f.outDir, profs...)
		if err != nil {
			return errors.Wrap(err, "failure in starting the profiler")
		}

		f.runningProfs = rp
	}
	return nil
}

func (f *profilerFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
}

func (f *profilerFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
}
