// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package trace provides common code to replay graphics trace files.
package trace

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

const (
	logDir  = "trace"
	envFile = "glxinfo.txt"
	replayAppAtHost = "/usr/local/cros_retrace"
	replayAppAtGuest = "/home/testuser/cros_retrace"
	fileServerPort = 8084
)

type TestEntry struct {
	Name string
	GsUrl string
	Size uint64
	Sha256sum string
	RepeatCount uint32
	CoolDownIntSec uint32
}

func logInfo(ctx context.Context, cont *vm.Container, file string) error {
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()

	cmd := cont.Command(ctx, "glxinfo")
	cmd.Stdout, cmd.Stderr = f, f
	return cmd.Run()
}

func GetOutboundIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String(), nil
}

// File server routine
type FileServer struct {
	ctx context.Context
	state *testing.State
}

func (server *FileServer)ServeHTTP(wr http.ResponseWriter, req *http.Request) {
  server.state.Log("[Proxy Server] Serving  request: ", req.URL.RawQuery)
	query, err := url.ParseQuery(req.URL.RawQuery)
	if err != nil {
		server.state.Fatalf("[Proxy Server] Unable to parse request query <%s>: %s\n", req.URL.RawQuery, err.Error())
	}

	gs_url, err := url.Parse(query["d"][0])
	if err != nil {
		server.state.Fatalf("[Proxy Server] Unable to parse gs url <%s>: %s\n", query["d"][0], err.Error())
	}

	cs := server.state.CloudStorage()
	r, err := cs.Open(server.ctx, gs_url.String())
	if err != nil {
		server.state.Fatalf("[Proxy Server] Unable to open <%v>: %v", gs_url, err)
	}
	defer r.Close()

	wr.Header().Set("Content-Disposition", "attachment; filename=aaa.trace")
	wr.WriteHeader(200)

  copied, err := io.Copy(wr, r)
	if err != nil {
		server.state.Fatal("[Proxy Server] io.Copy() failed: ", err)
	}
  server.state.Logf("[Proxy Server] Request served successfully. %d byte(s) copied.", copied)
}

func startFileServer(ctx context.Context, state *testing.State, addr string) *http.Server {
	handler := &FileServer{ ctx: ctx, state: state }
	state.Log("starting server at " + addr)
	srv := &http.Server {
		Addr: addr,
		Handler: handler,
	}
	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			state.Fatal("ListenAndServe() failed:", err)
		}
	}()

	return srv
}

// New RunTest
func RunTest2(ctx context.Context, state *testing.State, cont *vm.Container, entries []TestEntry) {
	// Gel outbound IP
	outbound_ip, err := GetOutboundIP()
	if err != nil {
		state.Fatalf("Unable to retreive outbound IP address: %v", err)
	}
	state.Log("Outbound IP address: ", outbound_ip)

	outDir := filepath.Join(state.OutDir(), logDir)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		state.Fatalf("Failed to create output dir %v: %v", outDir, err)
	}
	file := filepath.Join(outDir, envFile)
	state.Log("Logging container graphics environment to ", envFile)
	if err := logInfo(ctx, cont, file); err != nil {
		state.Log("Failed to log container information: ", err)
	}

	// check if replay app is exist
	if _, err := os.Stat(replayAppAtHost); os.IsNotExist(err) {
		state.Fatalf("Unable to locate replay app at host: <%s> not found!", replayAppAtHost)
	}

	// copy replay app into the container
	if err := cont.PushFile(ctx, replayAppAtHost, replayAppAtGuest); err != nil {
		state.Fatalf("Unable to copy replay app into the guest container: %v", err)
	}

	// start the file server
	file_server_addr := fmt.Sprintf("%s:%d", outbound_ip, fileServerPort)
	file_server := startFileServer(ctx, state, file_server_addr)

	shortCtx, shortCancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer shortCancel()
  for _, entry := range entries {
		perfValue, err := runTrace2(shortCtx, state, cont, file_server_addr, &entry)
		if err != nil {
			state.Fatal("Replay failed: ", err)
		}
		if err := perfValue.Save(state.OutDir()); err != nil {
			state.Fatal("Unable to save perf data: ", err)
		}
	}

	// shutdown the file server
	if err := file_server.Shutdown(ctx); err != nil {
		state.Fatal("Unable to shutdown file server:", err)
	}
}

type CrosRetraceResult struct {
	Result string `json:"result"`
	ErrorMessage string `json:"errorMessage"`
	Frames uint32 `json:"totalFrames,string"`
	Fps float32 `json:"averageFps,string"`
	Duration float32 `json:"durationInSeconds,string"`
}

func runTrace2(ctx context.Context, state *testing.State, cont *vm.Container, file_server_addr string, entry *TestEntry) (*perf.Values, error) {
	replay_args := fmt.Sprintf(
		`{"traceFile":{"proxy":"http://%s","gsUrl":"%s","size":"%d","sha256sum":"%s"},"testSettings":{"repeatCount":"%d","coolDownIntSec":"%d"}}`,
		file_server_addr, entry.GsUrl, entry.Size, entry.Sha256sum, entry.RepeatCount, entry.CoolDownIntSec)
	state.Log("Running replay with args: " + replay_args)
	replay_cmd := cont.Command(ctx, replayAppAtGuest, replay_args)
	replay_output, err := replay_cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	var result CrosRetraceResult
	if err := json.Unmarshal(replay_output, &result); err != nil {
    return nil, fmt.Errorf("Unable to parse: %s. Error: %v", string(replay_output), err)
	}

	if result.Result != "ok" {
		return nil, fmt.Errorf("Replay finished with the error: %s", result.ErrorMessage)
	}

	value := perf.NewValues()
	value.Set(perf.Metric{
		Name:      entry.Name,
		Variant:   "time",
		Unit:      "sec",
		Direction: perf.SmallerIsBetter,
	}, float64(result.Duration))
	value.Set(perf.Metric{
		Name:      entry.Name,
		Variant:   "frames",
		Unit:      "frame",
		Direction: perf.BiggerIsBetter,
	}, float64(result.Frames))
	value.Set(perf.Metric{
		Name:      entry.Name,
		Variant:   "fps",
		Unit:      "fps",
		Direction: perf.BiggerIsBetter,
	}, float64(result.Fps))
	return value, nil
}

// RunTest starts a VM and runs all traces in trace, which maps from filenames (passed to s.DataPath) to a human-readable name for the trace, that is used both for the output file's name and for the reported perf keyval.
func RunTest(ctx context.Context, s *testing.State, cont *vm.Container, traces map[string]string) {
	outDir := filepath.Join(s.OutDir(), logDir)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		s.Fatalf("Failed to create output dir %v: %v", outDir, err)
	}
	file := filepath.Join(outDir, envFile)
	s.Log("Logging container graphics environment to ", envFile)
	if err := logInfo(ctx, cont, file); err != nil {
		s.Log("Failed to log container information: ", err)
	}

	shortCtx, shortCancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer shortCancel()

	s.Log("Checking if apitrace installed")
	cmd := cont.Command(shortCtx, "sudo", "dpkg", "-l", "apitrace")
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(shortCtx)
		s.Fatal("Failed to get apitrace: ", err)
	}
	for traceFile, traceName := range traces {
		perfValues, err := runTrace(shortCtx, cont, s.DataPath(traceFile), traceName)
		if err != nil {
			s.Fatal("Failed running trace: ", err)
		}
		if err := perfValues.Save(s.OutDir()); err != nil {
			s.Fatal("Failed saving perf data: ", err)
		}
	}
}

// runTrace runs a trace and writes output to ${traceName}.txt. traceFile should be absolute path.
func runTrace(ctx context.Context, cont *vm.Container, traceFile, traceName string) (*perf.Values, error) {
	containerPath := filepath.Join("/tmp", filepath.Base(traceFile))
	if err := cont.PushFile(ctx, traceFile, containerPath); err != nil {
		return nil, errors.Wrap(err, "failed copying trace file to container")
	}

	containerPath, err := decompressTrace(ctx, cont, containerPath)
	if err != nil {
		return nil, err
	}

	testing.ContextLog(ctx, "Replaying trace file ", filepath.Base(containerPath))
	cmd := cont.Command(ctx, "apitrace", "replay", containerPath)
	traceOut, err := cmd.CombinedOutput()
	if err != nil {
		cmd.DumpLog(ctx)
		return nil, errors.Wrap(err, "failed to replay apitrace")
	}

	outDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return nil, errors.New("failed to get OutDir for writing trace result")
	}
	// Suggesting the file is human readable by appending txt extension.
	file := filepath.Join(outDir, logDir, traceName+".txt")
	testing.ContextLog(ctx, "Dumping trace output to file ", filepath.Base(file))
	if err := ioutil.WriteFile(file, traceOut, 0644); err != nil {
		return nil, errors.Wrap(err, "error writing tracing output")
	}
	return parseResult(traceName, string(traceOut))
}

// decompressTrace trys to decompress the trace into trace format if possible. If the input is uncompressed, this function will do nothing.
// Returns the uncompressed file absolute path.
func decompressTrace(ctx context.Context, cont *vm.Container, traceFile string) (string, error) {
	var decompressFile string
	testing.ContextLog(ctx, "Decompressing trace file ", traceFile)
	ext := filepath.Ext(traceFile)
	switch ext {
	case ".trace":
		decompressFile = traceFile
	case ".bz2":
		if _, err := cont.Command(ctx, "bunzip2", traceFile).CombinedOutput(testexec.DumpLogOnError); err != nil {
			return "", errors.Wrap(err, "failed to decompress bz2")
		}
		decompressFile = strings.TrimSuffix(traceFile, filepath.Ext(traceFile))
	case ".zst", ".xz":
		if _, err := cont.Command(ctx, "zstd", "-d", "-f", "--rm", "-T0", traceFile).CombinedOutput(testexec.DumpLogOnError); err != nil {
			return "", errors.Wrap(err, "failed to decompress zst")
		}
		decompressFile = strings.TrimSuffix(traceFile, filepath.Ext(traceFile))
	default:
		return "", errors.Errorf("unknown trace extension: %s", ext)
	}
	return decompressFile, nil
}

// parseResult parses the output of apitrace and return the perfs.
func parseResult(traceName, output string) (*perf.Values, error) {
	re := regexp.MustCompile(`Rendered (\d+) frames in (\d*\.?\d*) secs, average of (\d*\.?\d*) fps`)
	match := re.FindStringSubmatch(output)
	if match == nil {
		return nil, errors.New("result line can't be located")
	}

	frames, err := strconv.ParseUint(match[1], 10, 64)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse frames %q", match[1])
	}
	duration, err := strconv.ParseFloat(match[2], 64)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse duration %q", match[2])
	}
	fps, err := strconv.ParseFloat(match[3], 64)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse fps %q", match[3])
	}

	value := perf.NewValues()
	value.Set(perf.Metric{
		Name:      traceName,
		Variant:   "time",
		Unit:      "sec",
		Direction: perf.SmallerIsBetter,
	}, duration)
	value.Set(perf.Metric{
		Name:      traceName,
		Variant:   "frames",
		Unit:      "frame",
		Direction: perf.BiggerIsBetter,
	}, float64(frames))
	value.Set(perf.Metric{
		Name:      traceName,
		Variant:   "fps",
		Unit:      "fps",
		Direction: perf.BiggerIsBetter,
	}, fps)
	return value, nil
}
