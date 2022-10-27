// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"android.googlesource.com/platform/external/perfetto/protos/perfetto/trace/github.com/google/perfetto/perfetto_proto"
	"github.com/golang/protobuf/proto"
	"github.com/shirou/gopsutil/v3/process"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ash/ashproc"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/chromeproc"
	"chromiumos/tast/local/chrome/lacros"
	"chromiumos/tast/local/procutil"
	"chromiumos/tast/local/tracing"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PerfettoChromeProducer,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Tests Chrome connecting to the Perfetto system tracing service",
		Contacts:     []string{"chinglinyu@chromium.org", "baseos-perf@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			Val:     browser.TypeAsh,
			Fixture: "chromeLoggedIn",
		}, {
			Name:              "lacros",
			Val:               browser.TypeLacros,
			Fixture:           "lacros",
			ExtraSoftwareDeps: []string{"lacros", "lacros_stable"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "lacros_unstable",
			Val:               browser.TypeLacros,
			Fixture:           "lacros",
			ExtraSoftwareDeps: []string{"lacros", "lacros_unstable"},
			ExtraAttr:         []string{"informational"},
		}},
	})
}

// waitForChromeProducers waits until the Chrome producer processes are connected to the system tracing service daemon.
func waitForChromeProducers(ctx context.Context, f func() ([]*process.Process, error)) error {
	return testing.Poll(ctx, func(context.Context) error {
		// "perfetto --query-raw" outputs the TracingServiceState proto message.
		cmd := testexec.CommandContext(ctx, "/usr/bin/perfetto", "--query-raw")

		out, err := cmd.Output(testexec.DumpLogOnError)
		if err != nil {
			return errors.Wrap(err, "failed to query the service state of traced")
		}

		st := perfetto_proto.TracingServiceState{}
		// Count the number of producers named "org.chromium-(process ID)".
		if err = proto.Unmarshal(out, &st); err != nil {
			return errors.Wrap(err, "failed to parse the service state output of traced")
		}

		// Example chrome producer (in pbtxt):
		// producers: {
		//   id: 192
		//   name: "org.chromium-31550"
		//   uid: 1000
		//   sdk_version: "Perfetto v0.0 (unknown)"
		//   pid: 31400
		// }
		// Note that the real PID of the above example should be 31550. 31400 is the PID of the browser process, where the producer socket is connected.
		re := regexp.MustCompile(`^org.chromium-(\d+)$`)
		producers := make(map[int]bool)
		for _, prd := range st.GetProducers() {
			// Parse from the producer name to get the real PID. Don't use prd.GetPid().
			subs := re.FindStringSubmatch(prd.GetName())
			if subs == nil {
				continue // Not a Chrome producer.
			}
			pid, err := strconv.Atoi(subs[1])
			if err != nil {
				return errors.Errorf("failed to parse Chrome process from %s", prd.GetName())
			}
			producers[pid] = true
		}

		procs, err := f()
		if err != nil {
			return errors.Wrap(err, "failed to list chrome producer processes")
		}
		if len(procs) < 3 {
			return errors.Errorf("unexpected number of chrome producer processes, got: %d, want: >= 3", len(procs))
		}

		testing.ContextLog(ctx, "Checking producer processes: ", procs)

		// Compare the list of chrome producers from perfetto --query-raw and from process listing.
		// Require that each detected processes are connected to the tracing service.
		for _, p := range procs {
			if !producers[int(p.Pid)] {
				return errors.Errorf("chrome producer (pid=%d) not found", p.Pid)
			}
		}

		return nil
	}, &testing.PollOptions{
		// Chrome producers retry the connection with delay on failure to connect. Poll using a 30 second timeout.
		Timeout: 30 * time.Second,
	})
}

// lacrosProducerProcesses lists Lacros processes that should connect to the tracing service.
// This includes the browser, gpu and renderer processes. Utility, plugin, zygote and plugin processes are not listed.
func lacrosProducerProcesses() ([]*process.Process, error) {
	ptypeRe := regexp.MustCompile(`--type=(\S+)`)
	execRe := regexp.MustCompile(`^.*lacros.*chrome$`)
	lacrosProcessMatcher := func(p *process.Process) bool {
		// Can't use p.CmdlineSlice() because Chrome may rewrite its cmdline by replacing '\0' with ' '.
		// Join all cmdline args and then re-split with ' '. We only inspect arg 0 and 1 (exec and process type) so this is acceptable.
		cmd, err := p.Cmdline()
		if err != nil {
			return false
		}
		cmds := strings.Split(cmd, " ")
		if len(cmds) < 2 {
			// We'll inspect arg 0 and 1 in cmdline.
			return false
		}

		exec := cmds[0]
		// The executable must end with "chrome" and must contain "lacros"
		if !execRe.MatchString(exec) {
			return false
		}

		// This process runs the Lacros executable. Now filter by process type.
		ptype := cmds[1]
		subs := ptypeRe.FindStringSubmatch(ptype)
		if subs == nil {
			// Doesn't have process type: the browser process.
			return true
		}
		if subs[1] == "renderer" || subs[1] == "gpu-process" {
			return true
		}

		// Other Lacros process types are not checked.
		return false
	}

	return procutil.FindAll(lacrosProcessMatcher)
}

// ashProducerProcesses lists Ash processes that should connect to the tracing service.
// This includes the browser, gpu and renderer processes. Utility, plugin, zygote and plugin processes are not listed.
func ashProducerProcesses() ([]*process.Process, error) {
	// List ash processes.
	var procs []*process.Process
	// Add ash processes
	p, err := ashproc.Root()
	if err != nil {
		return nil, err
	}
	procs = append(procs, p)

	ps, err := chromeproc.GetGPUProcesses()
	if err != nil {
		return nil, err
	}
	procs = append(procs, ps...)

	ps, err = chromeproc.GetRendererProcesses()
	if err != nil {
		return nil, err
	}
	procs = append(procs, ps...)
	return procs, nil
}

// PerfettoChromeProducer tests Chrome as a perfetto trace producer.
// The test enables the "EnablePerfettoSystemTracing" feature flag for Chrome and then checks if traced sees multiple Chrome producers connected.
func PerfettoChromeProducer(ctx context.Context, s *testing.State) {
	_, _, err := tracing.CheckTracingServices(ctx)
	if err != nil {
		s.Fatal("Tracing services not running: ", err)
	}

	var f func() ([]*process.Process, error)
	lacrosBrowserType := s.Param().(browser.Type)
	// Launch Lacros to make it connect to the tracing service daemon.
	if lacrosBrowserType == browser.TypeLacros {
		_, l, _, err := lacros.Setup(ctx, s.FixtValue(), lacrosBrowserType)
		if err != nil {
			s.Fatal("Failed to setup lacros: ", err)
		}

		f = lacrosProducerProcesses
		defer lacros.CloseLacros(ctx, l)
	} else {
		f = ashProducerProcesses
	}

	if err = waitForChromeProducers(ctx, f); err != nil {
		s.Fatal("Failed in waiting for Chrome producers: ", err)
	}
}
