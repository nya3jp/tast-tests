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
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

const (
	outDirName = "trace"
	glxInfoFile = "glxinfo.txt"
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

func setTcpPortState(ctx context.Context, port int, open bool) error {
	iptablesApp := "iptables"
	iptablesArgs := []string { "INPUT", "-p", "tcp", "--dport", strconv.Itoa(port), "--syn", "-j", "ACCEPT"}

	checkCmd := testexec.CommandContext(ctx, iptablesApp, append([]string{"-C"}, iptablesArgs...)...)
	err := checkCmd.Run()
	exitCode, ok := testexec.ExitCode(err)
	if ok {
		var iptablesActionArg string
		if open == true && exitCode != 0 {
			iptablesActionArg = "-I"
		} else if open == false && exitCode == 0 {
			iptablesActionArg = "-D"
		} else {
			return nil
		}
		toggleCmd := testexec.CommandContext(ctx, iptablesApp, append([]string{iptablesActionArg}, iptablesArgs...)...)
		return  toggleCmd.Run()
	} else {
		if err != nil {
			return err
		} else {
			return fmt.Errorf("iptables failed")
		}
	}
}

// File server routine
type FileServer struct {
	ctx context.Context
	state *testing.State
}

func (server *FileServer)ServeHTTP(wr http.ResponseWriter, req *http.Request) {
	server.state.Log("[Proxy Server] Serving request: ", req.URL.RawQuery)
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

// RunTraceReplayTest starts a VM and replays all the traces in the test config
func RunTraceReplayTest(ctx context.Context, state *testing.State, cont *vm.Container, entries []TestEntryConfig) {
	// Gel outbound IP
	outboundIp, err := getOutboundIP()
	if err != nil {
		state.Fatalf("Unable to retreive outbound IP address: %v", err)
	}
	state.Log("Outbound IP address: ", outboundIp)

	// Open the file server port
	if err := setTcpPortState(ctx, fileServerPort, true); err != nil {
		state.Log("Unable to open TCP port: ", err)
	}

	defer func() {
		if err := setTcpPortState(ctx, fileServerPort, false); err != nil {
			state.Log("Unable to close TCP port: ", err)
		}
	}()

	outDir := filepath.Join(state.OutDir(), outDirName)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		state.Fatalf("Failed to create output dir %v: %v", outDir, err)
	}
	file := filepath.Join(outDir, glxInfoFile)
	state.Log("Logging container graphics environment to ", glxInfoFile)
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
	defer func() {
		if err := server.Shutdown(ctx); err != nil {
			state.Fatal("Unable to shutdown file server:", err)
		}
	}()

	shortCtx, shortCancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer shortCancel()
	for _, entry := range entries {
		entry.ProxyServer = ProxyServerInfo {
			Url: "http://" + serverAddr,
		}
		perfValue, err := replayTraceEntry(shortCtx, cont, &entry)
		if err != nil {
			state.Fatal("Replay failed: ", err)
		}
		if err := perfValue.Save(state.OutDir()); err != nil {
			state.Fatal("Unable to save perf data: ", err)
		}
	}
}

type ReplayResult struct {
	TotalFrames uint32 `json:"TotalFrames,string"`
	AverageFps float32 `json:"AverageFps,string"`
	DurationInSeconds float32 `json:"DurationInSeconds,string"`
}

type TestResult struct {
	Result string `json:"Result"`
	ErrorMessage string `json:"ErrorMessage"`
	Values []ReplayResult `json:"Values"`
}

// replayTraceEntry replays one trace entry inside the given VM instance
func replayTraceEntry(ctx context.Context, cont *vm.Container, entry *TestEntryConfig) (*perf.Values, error) {
	replayArgs, _ := json.Marshal(*entry)
	testing.ContextLog(ctx, "Running replay with args: " + string(replayArgs))
	replayCmd := cont.Command(ctx, replayAppAtGuest, string(replayArgs))
	replayOutput, err := replayCmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	testing.ContextLog(ctx,"reaply output: " + string(replayOutput))

	var testResult TestResult
	if err := json.Unmarshal(replayOutput, &testResult); err != nil {
		return nil, fmt.Errorf("Unable to parse: %s. Error: %v", string(replayOutput), err)
	}

	if testResult.Result != "ok" {
		return nil, fmt.Errorf("Replay finished with the error: %s", testResult.ErrorMessage)
	}

	type getFieldValueFn func(val ReplayResult) float64
	getValues := func(vals []ReplayResult, fn getFieldValueFn) []float64 {
		var values []float64
		for _, val := range vals {
			values = append(values, fn(val))
		}
		return values
	}

	perfValues := perf.NewValues()
	perfValues.Set(perf.Metric{
		Name:      entry.Name,
		Variant:   "time",
		Unit:      "sec",
		Direction: perf.SmallerIsBetter,
		Multiple: true,
	}, getValues(testResult.Values, func(r ReplayResult) float64 {
			return float64(r.DurationInSeconds);
		})...)
	perfValues.Set(perf.Metric{
		Name:      entry.Name,
		Variant:   "frames",
		Unit:      "frame",
		Direction: perf.BiggerIsBetter,
		Multiple: true,
	}, getValues(testResult.Values, func(r ReplayResult) float64 {
			return float64(r.TotalFrames);
		})...)
	perfValues.Set(perf.Metric{
		Name:      entry.Name,
		Variant:   "fps",
		Unit:      "fps",
		Direction: perf.BiggerIsBetter,
		Multiple: true,
	}, getValues(testResult.Values, func(r ReplayResult) float64 {
			return float64(r.AverageFps);
		})...)
	return perfValues, nil
}

