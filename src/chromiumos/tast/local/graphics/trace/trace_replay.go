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
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/graphics/trace/comm"
	"chromiumos/tast/lsbrelease"
	"chromiumos/tast/testing"
)

// IGuestOS interface is used to unify various VM guest OS access
type IGuestOS interface {
	// Command returns a testexec.Cmd with a vsh command that will run in the guest.
	Command(ctx context.Context, vshArgs ...string) *testexec.Cmd
	// GetBinPath returns the recommended binaries path in the guest OS.
	// The trace_replay binary will be uploaded into this directory.
	GetBinPath() string
	// GetTempPath returns the recommended temp path in the guest OS. This directory
	// can be used to store downloaded artifacts and other temp files.
	GetTempPath() string
}

const (
	outDirName          = "trace"
	glxInfoFile         = "glxinfo.txt"
	replayAppName       = "trace_replay"
	replayAppPathAtHost = "/usr/local/graphics"
	fileServerPort      = 8085
)

func logGuestInfo(ctx context.Context, guest IGuestOS, file string) error {
	f, err := os.Create(file)
	if err != nil {
		return errors.Wrapf(err, "Unable to create %s", file)
	}
	defer f.Close()

	cmd := guest.Command(ctx, "glxinfo", "-display", ":0")
	var errbuf strings.Builder
	cmd.Stdout, cmd.Stderr = f, &errbuf
	err = cmd.Run()
	if err != nil {
		return errors.Wrapf(err, "Unable to run glxinfo. Stderr: %s.", errbuf.String())
	}
	return nil
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
	repository   *comm.RepositoryInfo    // repository is a struct to communicate between guest and proxyServer.
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


type PassThruReader struct {
	io.Reader
	err error
	bytes int64
	usecs int64
}

func (pt *PassThruReader) Read(p []byte) (int, error) {
	startTime := time.Now()
	n, err := pt.Reader.Read(p)
	pt.usecs += time.Since(startTime).Microseconds()
	pt.bytes += int64(n)
	if err != nil {
		pt.err = err
	}
	return n, err
}

type PassThruWriter struct {
	io.Writer
	err error
	bytes int64
	usecs int64
}

func (pt *PassThruWriter) Write(p []byte) (int, error) {
	startTime := time.Now()
	n, err := pt.Writer.Write(p)
	pt.usecs += time.Since(startTime).Microseconds()
	pt.bytes += int64(n)
	if err != nil {
		pt.err = err
	}
	return n, err
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

	ptReader := &PassThruReader{Reader: r}
	ptWriter := &PassThruWriter{Writer: wr}

	copied, err := io.Copy(ptWriter, ptReader)
	if err != nil {
		s.log(ctx, "io.Copy failed")
	} else {
		s.log(ctx, "io.Copy finished successfully. %d bytes copied", copied)
	}

	statsString := func(e error, bytes int64, usecs int64) string {
		res := fmt.Sprintf("%d bytes in %d msec",
			bytes, usecs/1000);
		if e != nil && e != io.EOF {
			res += fmt.Sprintf(". Error: %s", e.Error());
		}
		return res
	}
	s.log(ctx, "Reader stats: %s", statsString(ptReader.err, ptReader.bytes, ptReader.usecs));
	s.log(ctx, "Writer stats: %s", statsString(ptWriter.err, ptWriter.bytes, ptWriter.usecs));

	if err != nil {
		return errors.Wrap(err, "io.Copy() failed")
	}
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
	case "getBinary":
		binaryFileName := path.Join(replayAppPathAtHost, replayAppName)
		if _, err := os.Stat(binaryFileName); os.IsNotExist(err) {
			s.log(ctx, "Error: unable to locate trace_replay app: <%s> not found", binaryFileName)
		}
		binaryFile, err := os.Open(binaryFileName)
		if err != nil {
			s.log(ctx, "Error: unable to open <%s>! %s", binaryFileName, err.Error())
		}
		defer binaryFile.Close()
		wr.Header().Set("Content-Disposition", "attachment; filename="+replayAppName)
		wr.WriteHeader(http.StatusOK)
		_, err = io.Copy(wr, binaryFile)
		if err != nil {
			s.log(ctx, "io.Copy() failed. %s", err.Error())
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

func pushTraceReplayApp(ctx context.Context, guest IGuestOS, serverIP string, serverPort int) error {
	if err := guest.Command(ctx, "mkdir", "-p", guest.GetBinPath()).Run(); err != nil {
		return errors.Wrapf(err, "unable to create directory <%s> inside the guest", guest.GetBinPath())
	}

	cmdLine := []string{
		"wget",
		"-O", path.Join(guest.GetBinPath(), replayAppName),
		fmt.Sprintf("http://%s:%d/?type=getBinary", serverIP, serverPort),
	}
	testing.ContextLogf(ctx, "Invoking %q", cmdLine)
	out, err := guest.Command(ctx, cmdLine...).CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "unable to upload trace_replay binary to the guest. %s", string(out))
	}

	out, err = guest.Command(ctx, "chmod", "+x", path.Join(guest.GetBinPath(), replayAppName)).CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "unable to chmod trace_replay binary. %s", string(out))
	}
	return nil
}

// grepGuestProcesses looks through the currently running processes on the guest and returns
// the comma separated list of process IDs which names exactly match the specified name
// (see pgrep manual for details)
func grepGuestProcesses(ctx context.Context, name string, guest IGuestOS) (string, error) {
	cmd := guest.Command(ctx, "pgrep", "-d,", "-x", name)
	out, err := cmd.Output()
	if err != nil {
		return "", errors.Wrap(err, "unable to invoke pgrep")
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// runTraceReplayInVM calls the trace_replay executable on the guest with args and writes test
// results to a file for Crosbolt to pick up later.
func runTraceReplayInVM(ctx context.Context, resultDir string, guest IGuestOS, group *comm.TestGroupConfig) error {
	replayArgs, err := json.Marshal(*group)
	if err != nil {
		return err
	}

	testing.ContextLog(ctx, "Running replay with args: "+string(replayArgs))
	replayCmd := guest.Command(ctx, path.Join(guest.GetBinPath(), replayAppName), string(replayArgs))
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

// runTraceReplayExtendedInVM calls the trace_replay executable on the guest with args.
// It also interacts with the graphics_Power subtest via IPC to create temporal
// checkpoints on the power-dashboard, and stores additional results to a directory
// to be collected by the managing server test.
func runTraceReplayExtendedInVM(ctx context.Context, resultDir string, guest IGuestOS, group *comm.TestGroupConfig) error {
	replayArgs, err := json.Marshal(*group)
	if err != nil {
		return err
	}

	testing.ContextLog(ctx, "Running extended replay with args: "+string(replayArgs))
	replayCmd := guest.Command(ctx, path.Join(guest.GetBinPath(), replayAppName), string(replayArgs))
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

// RunTraceReplayTest starts the VM and replays all the traces in the test config.
func RunTraceReplayTest(ctx context.Context, resultDir string, cloudStorage *testing.CloudStorage, guest IGuestOS, group *comm.TestGroupConfig, testVars *comm.TestVars) error {
	// Guest is unable to use the VM network interface to access it's host because of security reason,
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
	testing.ContextLog(ctx, "Logging guest OS graphics environment to ", glxInfoFile)
	if err := logGuestInfo(ctx, guest, file); err != nil {
		return errors.Wrap(err, "failed to log guest OS graphics environment information")
	}

	if err := getSystemInfo(&group.Host); err != nil {
		return err
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

	if err := pushTraceReplayApp(ctx, guest, outboundIP, fileServerPort); err != nil {
		return err
	}

	// Validate the protocol version of the guest trace_replay app
	replayAppVersionCmd := guest.Command(ctx, path.Join(guest.GetBinPath(), replayAppName), "--version")
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

	shortCtx, shortCancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer shortCancel()

	if deadline, ok := shortCtx.Deadline(); ok == true {
		seconds := deadline.Sub(time.Now()).Seconds() - 15
		if seconds < 0 {
			return errors.New("there is no time left to perform the test due to context deadline already exceeded")
		}
		group.Timeout = uint32(seconds)
	}

	group.ProxyServer = comm.ProxyServerInfo{
		URL: "http://" + serverAddr,
	}

	if group.ExtendedDuration > 0 {
		return runTraceReplayExtendedInVM(shortCtx, resultDir, guest, group)
	}
	err = runTraceReplayInVM(shortCtx, resultDir, guest, group)

	// If shortContext is timed out then it means the guest application probably hangs
	if errors.Is(err, context.DeadlineExceeded) {
		testing.ContextLogf(ctx, "WARNING: guest side %s execution context timed out", replayAppName)
		// Try to locate the hanged trace_replay processes and kill it and all its children
		// (glretrace, elgretrace, etc)
		pids, perr := grepGuestProcesses(ctx, replayAppName, guest)
		if perr != nil {
			testing.ContextLog(ctx, "WARNING: ", perr)
			err = errors.Wrap(err, perr.Error())
		} else {
			testing.ContextLogf(ctx, "Killing %s processes: %s", replayAppName, pids)
			if perr = guest.Command(ctx, "pkill", "-P", pids).Run(); perr != nil {
				testing.ContextLog(ctx, "WARNING: ", perr)
				err = errors.Wrap(err, perr.Error())
				// TODO(tutankhamen): We, probably, want to restart the VM or even reboot the DUT
			}
		}
	}

	return err

}
