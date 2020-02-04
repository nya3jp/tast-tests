// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package trace provides common code to replay graphics trace files.
package trace

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
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
	fileServerPort = 8085
)

type ProxyServerInfo struct{
Url string `json:"url"`
}

type FileInfo struct {
	GsUrl string `json:"gsUrl"`
	Size uint64 `json:"size,string"`
	Sha256sum string `json:"sha256sum"`
	Md5sum string `json: "md5sum"`
}

type TestSettings struct {
	RepeatCount uint32 `json:"repeatCount,string"`
	CoolDownIntSec uint32 `json:"coolDownIntSec,string"`
}

type TestEntryConfig struct {
  Name string `json:Name`
	ProxyServer ProxyServerInfo `json:"proxyServer"`
	StorageFile FileInfo `json:"storageFile"`
	TestSettings TestSettings `json:"testSettings"`
}

func logContainerInfo(ctx context.Context, cont *vm.Container, file string) error {
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()

	cmd := cont.Command(ctx, "glxinfo")
	cmd.Stdout, cmd.Stderr = f, f
	return cmd.Run()
}

func getOutboundIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String(), nil
}

func runCommand(name string, args ... string) (exitCode int, stdout string, stderr string) {
	var outbuf, errbuf bytes.Buffer
	var waitStatus syscall.WaitStatus
	cmd := exec.Command(name, args...)
	cmd.Stdout = &outbuf
	cmd.Stderr = &errbuf

	err := cmd.Run()
	stdout = outbuf.String()
	stderr = errbuf.String()

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			waitStatus = exitError.Sys().(syscall.WaitStatus)
			exitCode = waitStatus.ExitStatus()
		} else {
			exitCode = -1
			if stderr == "" {
				stderr = err.Error()
			}
		}
	} else {
		waitStatus = cmd.ProcessState.Sys().(syscall.WaitStatus)
		exitCode = waitStatus.ExitStatus()
	}
	return
}

func openTcpPort(port int, state *testing.State) error {
	if exitCode, stdout, stderr := runCommand("iptables", "-C", "INPUT", "-p", "tcp", "--dport", strconv.Itoa(port), "--syn", "-j", "ACCEPT"); exitCode != 0 {
		if exitCode == 1 {
			// insert the record into the ip table
			if exitCode, stdout, stderr := runCommand("iptables", "-I", "INPUT", "-p", "tcp", "--dport", strconv.Itoa(port), "--syn", "-j", "ACCEPT"); exitCode != 0 {
				return fmt.Errorf("iptables failed while inserting  with exit Code: %d. Stdout: %s, Stderr: %s", exitCode, stdout, stderr)
			}
		} else {
			return fmt.Errorf("iptables failed while checking with exit Code: %d. Stdout: %s, Stderr: %s", exitCode, stdout, stderr)
		}
	}
	return nil
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

	gsUrl, err := url.Parse(query["d"][0])
	if err != nil {
		server.state.Fatalf("[Proxy Server] Unable to parse gs url <%s>: %s\n", query["d"][0], err.Error())
	}

	cs := server.state.CloudStorage()
	r, err := cs.Open(server.ctx, gsUrl.String())
	if err != nil {
		server.state.Fatalf("[Proxy Server] Unable to open <%v>: %v", gsUrl, err)
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
  state.Log("Starting server at " + addr)
	server := &http.Server {
		Addr: addr,
		Handler: handler,
	}
	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			state.Fatal("ListenAndServe() failed:", err)
		}
	}()

	return server
}

// New RunTest
func RunTest2(ctx context.Context, state *testing.State, cont *vm.Container, entries []TestEntryConfig) {
	// Gel outbound IP
	outboundIp, err := getOutboundIP()
	if err != nil {
		state.Fatalf("Unable to retreive outbound IP address: %v", err)
	}
	state.Log("Outbound IP address: ", outboundIp)

	// Open the file server port
	if err := openTcpPort(fileServerPort, state); err != nil {
		state.Log("Unable to open TCP port: ", err)
	}

	outDir := filepath.Join(state.OutDir(), logDir)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		state.Fatalf("Failed to create output dir %v: %v", outDir, err)
	}
	file := filepath.Join(outDir, envFile)
	state.Log("Logging container graphics environment to ", envFile)
	if err := logContainerInfo(ctx, cont, file); err != nil {
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
	serverAddr := fmt.Sprintf("%s:%d", outboundIp, fileServerPort)
	server := startFileServer(ctx, state, serverAddr)

	shortCtx, shortCancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer shortCancel()
  for _, entry := range entries {
		entry.ProxyServer = ProxyServerInfo {
			Url: "http://" + serverAddr,
		}
		perfValue, err := runTrace2(shortCtx, cont, &entry)
		if err != nil {
			state.Fatal("Replay failed: ", err)
		}
		if err := perfValue.Save(state.OutDir()); err != nil {
			state.Fatal("Unable to save perf data: ", err)
		}
	}

	// shutdown the file server
	if err := server.Shutdown(ctx); err != nil {
		state.Fatal("Unable to shutdown file server:", err)
	}
	// TODO: Do we want to delete the proxy server iptables record afterwards?
}

type CrosRetraceResult struct {
	Result string `json:"result"`
	ErrorMessage string `json:"errorMessage"`
	Frames uint32 `json:"totalFrames,string"`
	Fps float32 `json:"averageFps,string"`
	Duration float32 `json:"durationInSeconds,string"`
}

func runTrace2(ctx context.Context, cont *vm.Container, entry *TestEntryConfig) (*perf.Values, error) {
	replayArgs, _ := json.Marshal(*entry)
	testing.ContextLog(ctx, "Running replay with args: " + string(replayArgs))
	replayCmd := cont.Command(ctx, replayAppAtGuest, string(replayArgs))
	replayOutput, err := replayCmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	var result CrosRetraceResult
	if err := json.Unmarshal(replayOutput, &result); err != nil {
		return nil, fmt.Errorf("Unable to parse: %s. Error: %v", string(replayOutput), err)
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
	if err := logContainerInfo(ctx, cont, file); err != nil {
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
