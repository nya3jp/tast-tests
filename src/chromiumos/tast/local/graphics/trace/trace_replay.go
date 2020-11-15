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
	"path"
	"path/filepath"
	"strconv"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/graphics/trace/comm"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/lsbrelease"
	"chromiumos/tast/testing"
)

const (
	outDirName           = "trace"
	glxInfoFile          = "glxinfo.txt"
	replayAppName        = "trace_replay"
	replayAppPathAtHost  = "/usr/local/graphics"
	replayAppPathAtGuest = "/tmp/graphics"
	fileServerPort       = 8085
)

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

func getSystemInfo(sysInfo *comm.SystemInfo) error {
	lsbReleaseData, err := lsbrelease.Load()
	if err != nil {
		return errors.Wrap(err, "unable to retreive lsbrelease information")
	}

	sysInfo.Board = lsbReleaseData[lsbrelease.Board]
	sysInfo.ChromeOSVersion = lsbReleaseData[lsbrelease.Version]
	return nil
}

func getOutboundIP() (string, error) {
	// 8.8.8.8 is used as a known WAN ip address to get the proper network interface
	// for outbound connections. No actual connection is estabilished.
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String(), nil
}

func setTCPPortState(ctx context.Context, port int, open bool) error {
	const iptablesApp = "iptables"
	iptablesArgs := []string{"INPUT", "-p", "tcp", "--dport", strconv.Itoa(port), "--syn", "-j", "ACCEPT"}
	checkCmd := testexec.CommandContext(ctx, iptablesApp, append([]string{"-C"}, iptablesArgs...)...)
	err := checkCmd.Run()
	exitCode, ok := testexec.ExitCode(err)
	if !ok {
		return err
	}
	var iptablesActionArg string
	if open == true && exitCode != 0 {
		iptablesActionArg = "-I"
	} else if open == false && exitCode == 0 {
		iptablesActionArg = "-D"
	} else {
		return nil
	}
	toggleCmd := testexec.CommandContext(ctx, iptablesApp, append([]string{iptablesActionArg}, iptablesArgs...)...)
	return toggleCmd.Run()
}

// graphicsPowerInterface provides control of the graphics_Power logging subtest.
type graphicsPowerInterface struct {
	// signalRunningFile is a file that controls/exists when the graphics_Power test is running.
	signalRunningFile string
	// signalCheckpointFile is a file that the graphics_Power test listens to for creating new checkpoints.
	signalCheckpointFile string
}

// stop deletes signalRunningFile monitored by the graphics_Power process, informing it to shutdown gracefully
func (gpi *graphicsPowerInterface) stop(ctx context.Context) error {
	if err := os.Remove(gpi.signalRunningFile); err != nil {
		testing.ContextLogf(ctx, "Failed to remove stop signal file %s to shutdown graphics_Power test process", gpi.signalRunningFile)
		return err
	}
	return nil
}

// finishCheckpointWithStartTime writes to signalCheckpointFile monitored by the graphics_Power process, informing it to save a checkpoint.
// The passed startTime as seconds since the epoch is used as the checkpoint's start time.
func (gpi *graphicsPowerInterface) finishCheckpointWithStartTime(ctx context.Context, name string, startTime float64) error {
	if err := ioutil.WriteFile(gpi.signalCheckpointFile, []byte(name+"\n"+strconv.FormatFloat(startTime, 'e', -1, 64)), 0644); err != nil {
		testing.ContextLogf(ctx, "Failed to write graphics_Power checkpoint signal file %s", gpi.signalCheckpointFile)
		return err
	}
	return nil
}

// finishCheckpoint writes to signalCheckpointFile monitored by the graphics_Power process, informing it to save a checkpoint.
// The current checkpoint is started immediately after the previous checkpoint's end.
func (gpi *graphicsPowerInterface) finishCheckpoint(ctx context.Context, name string) error {
	if err := ioutil.WriteFile(gpi.signalCheckpointFile, []byte(name), 0644); err != nil {
		testing.ContextLogf(ctx, "Failed to write graphics_Power checkpoint signal file %s", gpi.signalCheckpointFile)
		return err
	}
	return nil
}

// File server routine. It serves all the artifact requests request from the guest.
type fileServer struct {
	cloudStorage *testing.CloudStorage   // cloudStorage is a client to read files on Google Cloud Storage.
	repository   *comm.RepositoryInfo    // repository is a struct to communicate between container and proxyServer.
	outDir       string                  // outDir is directory to store the received file.
	gpi          *graphicsPowerInterface // gpi is an interface for issuing IPC signals to the graphics_Power test process.
}

func validateRequestedFilePath(filePath string) bool {
	// detect dot segments in filePath using path.Join()
	return path.Join("/", filePath)[1:] == filePath
}

// log logs the information to tast log.
func (s *fileServer) log(ctx context.Context, format string, args ...interface{}) {
	testing.ContextLogf(ctx, "[Proxy Server] "+format, args...)
}

// serveDownloadRequest tries to download a file and transmit it via the http response.
func (s *fileServer) serveDownloadRequest(ctx context.Context, wr http.ResponseWriter, filePath string) error {
	// The requested path is specified relative to the repository root URL to restrict access to arbitrary files via this request.
	s.log(ctx, "Validate requested file path: %s", filePath)
	if !validateRequestedFilePath(filePath) {
		wr.WriteHeader(http.StatusUnauthorized)
		return errors.Errorf("unable to validate the requested path %v", filePath)
	}

	s.log(ctx, "Parse repository URL %s", s.repository.RootURL)
	requestURL, err := url.Parse(s.repository.RootURL)
	if err != nil {
		wr.WriteHeader(http.StatusBadRequest)
		return errors.Wrap(err, "unable to parse the repository URL")
	}

	requestURL.Path = path.Join(requestURL.Path, filePath)
	s.log(ctx, "Downloading: %s", requestURL)
	r, err := s.cloudStorage.Open(ctx, requestURL.String())
	if err != nil {
		wr.WriteHeader(http.StatusNotFound)
		return errors.Wrap(err, "unable to download")
	}
	defer r.Close()

	wr.Header().Set("Content-Disposition", "attachment; filename="+path.Base(filePath))
	wr.WriteHeader(http.StatusOK)

	copied, err := io.Copy(wr, r)
	if err != nil {
		return errors.Wrap(err, "io.Copy() failed")
	}
	s.log(ctx, "%d byte(s) copied", copied)
	return nil
}

// ServeHTTP implements http.HandlerFunc interface to serve incoming request.
func (s *fileServer) ServeHTTP(wr http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	query, err := url.ParseQuery(req.URL.RawQuery)
	if err != nil {
		s.log(ctx, "Error: Unable to parse request query: %s", err.Error())
		wr.WriteHeader(http.StatusBadRequest)
		return
	}

	// The server serves many types of requests, one at a time, determined by the string value of the request's "type" value.
	// Each request type can make use of any number of additional values by matching pre-determined key strings for their values.
	requestType := query.Get("type")
	switch requestType {
	case "download":
		filePath := query.Get("filePath")
		if filePath == "" {
			s.log(ctx, "Error: filePath was not provided by download request to proxyServer")
			wr.WriteHeader(http.StatusBadRequest)
			return
		}
		if err := s.serveDownloadRequest(ctx, wr, filePath); err != nil {
			s.log(ctx, "serveDownloadRequest failed: ", err.Error())
		}
	case "log":
		message := query.Get("message")
		if message == "" {
			s.log(ctx, "Error: message was not provided by log request to proxyServer")
			wr.WriteHeader(http.StatusBadRequest)
			return
		}
		s.log(ctx, message)
	case "notifyInitFinished":
		s.log(ctx, "Test initialization has finished")
		if s.gpi != nil {
			s.gpi.finishCheckpoint(ctx, "Initialization")
		}
	case "notifyReplayFinished":
		replayDesc := query.Get("replayDescription")
		if replayDesc == "" {
			s.log(ctx, "Error: replayDescription was not provided by notifyReplayFinished request to proxyServer")
			wr.WriteHeader(http.StatusBadRequest)
			return
		}
		s.log(ctx, "A trace replay %s has finished", replayDesc)
		if s.gpi != nil {
			replayStartTime := query.Get("replayStartTime")
			if replayStartTime == "" {
				s.log(ctx, "Error: replayStartTime was not provided by notifyReplayFinished request to proxyServer")
				wr.WriteHeader(http.StatusBadRequest)
				return
			}
			startTime, err := strconv.ParseFloat(replayStartTime, 64)
			if err != nil {
				s.log(ctx, "replayStartTime parsing to float failed: ", err.Error())
				return
			}
			s.gpi.finishCheckpointWithStartTime(ctx, replayDesc, startTime)
		}
	default:
		s.log(ctx, "Skip request: %v", requestType)
	}
	wr.WriteHeader(http.StatusOK)
}

func startFileServer(ctx context.Context, addr, outDir string, cloudStorage *testing.CloudStorage, repository *comm.RepositoryInfo, gpi *graphicsPowerInterface) *http.Server {
	handler := &fileServer{cloudStorage: cloudStorage, repository: repository, outDir: outDir, gpi: gpi}
	testing.ContextLog(ctx, "Starting server at "+addr)
	server := &http.Server{
		Addr:    addr,
		Handler: handler,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
	}
	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			testing.ContextLog(ctx, "Error: ListenAndServe() failed: ", err)
		}
	}()
	return server
}

func pushTraceReplayApp(ctx context.Context, cont *vm.Container) error {
	replayAppAtHost := path.Join(replayAppPathAtHost, replayAppName)
	if _, err := os.Stat(replayAppAtHost); os.IsNotExist(err) {
		return errors.Wrapf(err, "unable to locate replay app at host: <%s> not found", replayAppAtHost)
	}

	if err := cont.Command(ctx, "mkdir", "-p", replayAppPathAtGuest).Run(); err != nil {
		return errors.Wrapf(err, "unable to create directory <%s> inside the container", replayAppPathAtGuest)
	}

	replayAppAtGuest := path.Join(replayAppPathAtGuest, replayAppName)
	testing.ContextLogf(ctx, "Copying %s to the container <%s>", replayAppName, replayAppPathAtGuest)
	if err := cont.PushFile(ctx, replayAppAtHost, replayAppAtGuest); err != nil {
		return errors.Wrap(err, "unable to copy replay app into the guest container")
	}
	return nil
}

// runTraceReplayInVM calls the trace_replay executable in a VM with args and writes test results to a file
// for Crosbolt to pick up later.
func runTraceReplayInVM(ctx context.Context, resultDir string, cont *vm.Container, group *comm.TestGroupConfig) error {
	replayArgs, err := json.Marshal(*group)
	if err != nil {
		return err
	}

	testing.ContextLog(ctx, "Running replay with args: "+string(replayArgs))
	replayCmd := cont.Command(ctx, path.Join(replayAppPathAtGuest, replayAppName), string(replayArgs))
	replayOutput, err := replayCmd.Output()
	if err != nil {
		return err
	}

	testing.ContextLog(ctx, "Replay output: "+string(replayOutput))

	var testResult comm.TestGroupResult
	if err := json.Unmarshal(replayOutput, &testResult); err != nil {
		return errors.Wrapf(err, "unable to parse test group result output: %q", string(replayOutput))
	}
	getDirection := func(d int32) perf.Direction {
		if d < 0 {
			return perf.SmallerIsBetter
		}
		return perf.BiggerIsBetter
	}

	perfValues := perf.NewValues()
	failedEntries := 0
	for _, resultEntry := range testResult.Entries {
		if resultEntry.Message != "" {
			testing.ContextLog(ctx, resultEntry.Message)
		}
		if resultEntry.Result != comm.TestResultSuccess {
			failedEntries++
			continue
		}
		for key, value := range resultEntry.Values {
			perfValues.Set(perf.Metric{
				Name:      resultEntry.Name,
				Variant:   key,
				Unit:      value.Unit,
				Direction: getDirection(value.Direction),
				Multiple:  false,
			}, float64(value.Value))
		}
	}

	if err := perfValues.Save(resultDir); err != nil {
		return errors.Wrap(err, "unable to save performance values")
	}

	if testResult.Result != comm.TestResultSuccess {
		return errors.Errorf("%s", testResult.Message)
	}

	return nil
}

// runTraceReplayExtendedInVM calls the trace_replay executable in a VM with args.
// It also interacts with the graphics_Power subtest via IPC to create temporal
// checkpoints on the power-dashboard, and stores additional results to a directory
// to be collected by the managing server test.
func runTraceReplayExtendedInVM(ctx context.Context, resultDir string, cont *vm.Container, group *comm.TestGroupConfig) error {
	replayArgs, err := json.Marshal(*group)
	if err != nil {
		return err
	}

	testing.ContextLog(ctx, "Running extended replay with args: "+string(replayArgs))
	replayCmd := cont.Command(ctx, path.Join(replayAppPathAtGuest, replayAppName), string(replayArgs))
	replayOutput, err := replayCmd.Output()
	if err != nil {
		return err
	}

	testing.ContextLog(ctx, "Replay output: "+string(replayOutput))

	var testResult comm.TestGroupResult
	if err := json.Unmarshal(replayOutput, &testResult); err != nil {
		return errors.Wrapf(err, "unable to parse test group result output: %q", string(replayOutput))
	}

	if testResult.Result != comm.TestResultSuccess {
		return errors.Errorf("%s", testResult.Message)
	}

	return nil
}

// RunTraceReplayTest starts a VM and replays all the traces in the test config.
func RunTraceReplayTest(ctx context.Context, resultDir string, cloudStorage *testing.CloudStorage, cont *vm.Container, group *comm.TestGroupConfig, testVars *comm.TestVars) error {
	// Guest is unable to use VM network interface to access it's host because of security reason,
	// and the only to make such connectivity is to use host's outbound network interface.
	outboundIP, err := getOutboundIP()
	if err != nil {
		return errors.Wrap(err, "unable to retrieve outbound IP address")
	}
	testing.ContextLog(ctx, "Outbound IP address: ", outboundIP)

	if err := setTCPPortState(ctx, fileServerPort, true); err != nil {
		return errors.Wrap(err, "unable to open TCP port")
	}

	defer func() {
		if err := setTCPPortState(ctx, fileServerPort, false); err != nil {
			testing.ContextLog(ctx, "Unable to close TCP port: ", err)
		}
	}()

	outDir := filepath.Join(resultDir, outDirName)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return errors.Wrapf(err, "failed to create output dir <%v>", outDir)
	}
	file := filepath.Join(outDir, glxInfoFile)
	testing.ContextLog(ctx, "Logging container graphics environment to ", glxInfoFile)
	if err := logContainerInfo(ctx, cont, file); err != nil {
		return errors.Wrap(err, "failed to log container information")
	}

	if err := getSystemInfo(&group.Host); err != nil {
		return err
	}

	if err := pushTraceReplayApp(ctx, cont); err != nil {
		return err
	}

	// Validate the protocol version of the guest trace_replay app
	replayAppVersionCmd := cont.Command(ctx, path.Join(replayAppPathAtGuest, replayAppName), "--version")
	replayAppVersionOutput, err := replayAppVersionCmd.Output()
	if err != nil {
		return err
	}
	var replayAppVersionInfo comm.VersionInfo
	if err := json.Unmarshal(replayAppVersionOutput, &replayAppVersionInfo); err != nil {
		return errors.Wrapf(err, "unable to parse trace_replay --version output: %q ", string(replayAppVersionOutput))
	}
	if replayAppVersionInfo.ProtocolVersion != comm.ProtocolVersion {
		return errors.Errorf("trace_replay protocol version mismatch. Host version: %d. Guest version: %d. Please make sure to sync the chroot/tast bundle to the same revision as the DUT image", comm.ProtocolVersion, replayAppVersionInfo.ProtocolVersion)
	}

	var gpi *graphicsPowerInterface
	if group.ExtendedDuration > 0 {
		gpi = &graphicsPowerInterface{signalRunningFile: testVars.PowerTestVars.SignalRunningFile, signalCheckpointFile: testVars.PowerTestVars.SignalCheckpointFile}

	}
	serverAddr := fmt.Sprintf("%s:%d", outboundIP, fileServerPort)
	server := startFileServer(ctx, serverAddr, outDir, cloudStorage, &group.Repository, gpi)
	defer func() {
		if err := server.Shutdown(ctx); err != nil {
			testing.ContextLog(ctx, "Unable to shutdown file server: ", err)
		}
	}()
	shortCtx, shortCancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer shortCancel()

	group.ProxyServer = comm.ProxyServerInfo{
		URL: "http://" + serverAddr,
	}

	if group.ExtendedDuration > 0 {
		return runTraceReplayExtendedInVM(shortCtx, resultDir, cont, group)
	}
	return runTraceReplayInVM(shortCtx, resultDir, cont, group)
}
