// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package trace provides common code to replay graphics trace files.
package trace

import (
	"bufio"
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
	outDirName           = "guest"
	glxInfoFile          = "glxinfo.txt"
	replayAppName        = "trace_replay"
	replayAppPathAtHost  = "/usr/local/graphics"
	fileServerPort       = 8085
	defaultMaxBufferSize = 32 << 20 // 32 MB
)

var unwantedBoardSuffixes = []string{
	"-borealis",
}

type guestLogEntry struct {
	entryName   string
	logFileName string
	command     []string
}

var preRunGuestLogEntryList = []guestLogEntry{
	{
		entryName:   "glxinfo output",
		logFileName: "glxinfo.txt",
		command:     []string{"glxinfo", "-display", ":0"},
	},
	{
		entryName:   "dpkg output",
		logFileName: "dpkg.txt",
		command:     []string{"dpkg", "-l"},
	},
	{
		entryName:   "pacman output",
		logFileName: "pacman.txt",
		command:     []string{"pacman", "-Q"},
	},
	{
		entryName:   "lsb-release",
		logFileName: "lsb-release.txt",
		command:     []string{"cat", "/etc/lsb-release"},
	},
}

var postRunGuestLogEntryList = []guestLogEntry{
	{
		entryName:   "dmesg output",
		logFileName: "dmesg.log",
		command:     []string{"dmesg"},
	},
}

func logGuestCommand(ctx context.Context, guest IGuestOS, commandArgs []string, logFile string) error {
	f, err := os.Create(logFile)
	if err != nil {
		return errors.Wrapf(err, "unable to create %s", logFile)
	}
	defer f.Close()

	cmd := guest.Command(ctx, commandArgs...)
	cmd.Stdout, cmd.Stderr = f, f
	err = cmd.Run()
	if err != nil {
		return errors.Wrapf(err, "unable to run glxinfo (check %s for stderr output)", logFile)
	}
	return nil
}

func getSystemInfo(sysInfo *comm.SystemInfo) error {
	lsbReleaseData, err := lsbrelease.Load()
	if err != nil {
		return errors.Wrap(err, "unable to retreive lsbrelease information")
	}

	sysInfo.Board = lsbReleaseData[lsbrelease.Board]
	// trim unwanted suffixes in board name
	for _, suffix := range unwantedBoardSuffixes {
		if strings.HasSuffix(sysInfo.Board, suffix) {
			trimmedBoardName := sysInfo.Board[:len(sysInfo.Board)-len(suffix)]
			sysInfo.Board = trimmedBoardName
		}
	}

	sysInfo.ChromeOSVersion = lsbReleaseData[lsbrelease.Version]
	return nil
}

func modifyHostSwappiness(ctx context.Context, tuneValue uint32) error {
	return testexec.CommandContext(ctx, "sudo", "sysctl", "vm.swappiness="+strconv.FormatUint(uint64(tuneValue), 10)).Run()
}

func readHostSwappiness() (uint32, error) {
	path := "/proc/sys/vm/swappiness"
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	var swappiness string
	for scanner.Scan() {
		fmt.Sprintf("loading Swappiness: %s", scanner.Text())
		swappiness = scanner.Text()
		break
	}
	convert, err := strconv.ParseUint(swappiness, 10, 32)
	return uint32(convert), nil
}

func setupSwappiness(ctx context.Context, tuneValue uint32) error {
	// Read current swappiness.
	swappinessValueCurrent, err := readHostSwappiness()
	if err != nil {
		return errors.Wrap(err, "failed to get swappiness value")
	}
	testing.ContextLog(ctx, "Current swappiness value is: ", swappinessValueCurrent)
	testing.ContextLog(ctx, "Target swappiness is:", tuneValue)
	// Modify swappiness to tuneValue.
	if tuneValue > 0 {
		if err := modifyHostSwappiness(ctx, tuneValue); err != nil {
			return errors.Wrap(err, "failed to modify swappiness value")
		}
	}
	// Read it again for validation.
	swappinessValueModified, err := readHostSwappiness()
	if err != nil {
		return errors.Wrap(err, "failed to get swappiness value")
	}
	testing.ContextLog(ctx, "After update, swappiness value is: ", swappinessValueModified)
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
		return errors.Wrap(err, "failed to query iptable settings")
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

type passThruReader struct {
	io.Reader
	err   error
	bytes int64
	usecs int64
}

func (pt *passThruReader) Read(p []byte) (int, error) {
	startTime := time.Now()
	n, err := pt.Reader.Read(p)
	pt.usecs += time.Since(startTime).Microseconds()
	pt.bytes += int64(n)
	if err != nil {
		pt.err = err
	}
	return n, err
}

type passThruWriter struct {
	io.Writer
	err   error
	bytes int64
	usecs int64
}

func (pt *passThruWriter) Write(p []byte) (int, error) {
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
	s.log(ctx, "serveDownloadRequest() for [%s]", filePath)
	if filePath == "" {
		wr.WriteHeader(http.StatusBadRequest)
		return errors.New("serveDownloadRequest: no filePath argument is provided")
	}

	if !validateRequestedFilePath(filePath) {
		wr.WriteHeader(http.StatusUnauthorized)
		return errors.Errorf("serveDownloadRequest: invalid requested path %v", filePath)
	}

	s.log(ctx, "Parse repository URL %s", s.repository.RootURL)
	requestURL, err := url.Parse(s.repository.RootURL)
	if err != nil {
		wr.WriteHeader(http.StatusBadRequest)
		return errors.Wrap(err, "serveDownloadRequest: unable to parse the repository URL")
	}

	requestURL.Path = path.Join(requestURL.Path, filePath)
	s.log(ctx, "Downloading: %s", requestURL)
	r, err := s.cloudStorage.Open(ctx, requestURL.String())
	if err != nil {
		wr.WriteHeader(http.StatusNotFound)
		return errors.Wrap(err, "serveDownloadRequest: Failed cloudStorage.Open()")
	}
	defer r.Close()

	wr.Header().Set("Content-Disposition", "attachment; filename="+path.Base(filePath))
	wr.WriteHeader(http.StatusOK)

	ptReader := &passThruReader{Reader: r}
	ptWriter := &passThruWriter{Writer: wr}

	copied, err := io.Copy(ptWriter, ptReader)
	if err != nil {
		s.log(ctx, "io.Copy failed. %s", err.Error())
	} else {
		s.log(ctx, "io.Copy finished successfully. %d bytes copied", copied)
	}

	formatStats := func(e error, bytes, usecs int64) string {
		res := fmt.Sprintf("%d bytes in %d msec", bytes, usecs/1000)
		if e != nil && e != io.EOF {
			res += fmt.Sprintf(". Error: %s", e.Error())
		}
		return res
	}
	s.log(ctx, "Reader stats: %s", formatStats(ptReader.err, ptReader.bytes, ptReader.usecs))
	s.log(ctx, "Writer stats: %s", formatStats(ptWriter.err, ptWriter.bytes, ptWriter.usecs))

	if err != nil {
		return errors.Wrap(err, "serveDownloadRequest: Failed io.Copy()")
	}
	return nil
}

// serveUploadRequest serves an upload request from the guest and stores the received files into the folder with test result artifacts
func (s *fileServer) serveUploadRequest(ctx context.Context, wr http.ResponseWriter, req *http.Request) error {
	s.log(ctx, "serveUploadRequest()")
	if err := req.ParseMultipartForm(defaultMaxBufferSize); err != nil {
		wr.WriteHeader(http.StatusBadRequest)
		return errors.Wrap(err, "serveUploadRequest: Failed to parse request as a multipart-form")
	}

	if req.MultipartForm == nil || len(req.MultipartForm.File) == 0 {
		wr.WriteHeader(http.StatusBadRequest)
		return errors.New("serveUploadRequest: Empty multipart request")
	}

	wr.WriteHeader(http.StatusOK)
	fileHeaders := req.MultipartForm.File["file"]
	if len(fileHeaders) == 0 {
		return errors.New("serveUploadRequest: No file entries in the multipart request")
	}
	for _, fileHeader := range fileHeaders {
		outFileName := filepath.Join(s.outDir, fileHeader.Filename)
		s.log(ctx, "Receiving %s...", fileHeader.Filename)
		if err := os.MkdirAll(filepath.Dir(outFileName), 0755); err != nil {
			return errors.Wrapf(err, "serveUploadRequest: Failed os.MkdirAll %v", filepath.Dir(outFileName))
		}
		body, err := fileHeader.Open()
		defer body.Close()
		if err != nil {
			return errors.Wrap(err, "serveUploadRequest: Failed fileHeader.Open()")
		}

		file, err := os.Create(outFileName)
		defer file.Close()
		if err != nil {
			return errors.Wrap(err, "serveUploadRequest: Failed os.Create()")
		}

		copied, err := io.Copy(file, body)
		if err != nil {
			return errors.Wrap(err, "serveUploadRequest: io.Copy() failed")
		}
		s.log(ctx, "Saved %d bytes to %s", copied, outFileName)
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
		if err := s.serveDownloadRequest(ctx, wr, query.Get("filePath")); err != nil {
			s.log(ctx, "Error: serveDownloadRequest failed: ", err.Error())
		}
		return
	case "upload":
		if err := s.serveUploadRequest(ctx, wr, req); err != nil {
			s.log(ctx, "Error: serveUploadRequest failed: ", err.Error())
		}
		return
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

func startFileServer(ctx context.Context, addr, outDir string, cloudStorage *testing.CloudStorage, repository *comm.RepositoryInfo, gpi *graphicsPowerInterface) (*http.Server, error) {
	handler := &fileServer{cloudStorage: cloudStorage, repository: repository, outDir: outDir, gpi: gpi}
	testing.ContextLog(ctx, "Starting server at "+addr)
	server := &http.Server{
		Addr:    addr,
		Handler: handler,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
	}
	listener, err := net.Listen("tcp", server.Addr)
	// The server will close the listener when it closes, so no need to defer that.
	if err != nil {
		return nil, errors.Wrapf(err, "failed to listen on %v", server.Addr)
	}
	go func() {
		if err := server.Serve(listener); err != http.ErrServerClosed {
			testing.ContextLog(ctx, "Error: ListenAndServe() failed: ", err)
		}
	}()
	return server, nil
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
	if out, err := guest.Command(ctx, cmdLine...).CombinedOutput(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "unable to upload trace_replay binary to the guest. %s", string(out))
	}

	if out, err := guest.Command(ctx, "chmod", "+x", path.Join(guest.GetBinPath(), replayAppName)).CombinedOutput(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "unable to chmod trace_replay binary. %s", string(out))
	}
	return nil
}

// grepGuestProcesses looks through the currently running processes on the guest and returns
// the comma separated list of process IDs which names exactly match the specified name
// (see pgrep manual for details)
func grepGuestProcesses(ctx context.Context, name string, guest IGuestOS) (string, error) {
	out, err := guest.Command(ctx, "pgrep", "-d,", "-x", name).Output(testexec.DumpLogOnError)
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
		return errors.Wrap(err, "failed to marshal TestGroupConfig")
	}

	testing.ContextLog(ctx, "Running replay with args: "+string(replayArgs))
	replayOutput, err := guest.Command(ctx, path.Join(guest.GetBinPath(), replayAppName), string(replayArgs)).Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed while running trace_replay")
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
		return errors.Wrap(err, "failed to marshal TestGroupConfig")
	}

	testing.ContextLog(ctx, "Running extended replay with args: "+string(replayArgs))
	replayOutput, err := guest.Command(ctx, path.Join(guest.GetBinPath(), replayAppName), string(replayArgs)).Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed while running trace_replay")
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

// runTraceReplayTuningInVM first setup the configuration to be tuned for the DUT
// e.g. swappiness, then runs trace_replay multiple times on guest with `replayArgs`,
// and generate a report of average FPS for the configuration over multiple runs.
func runTraceReplayTuningInVM(ctx context.Context, resultDir string, guest IGuestOS, group *comm.TestGroupConfig) error {
	testing.ContextLog(ctx, "Extended Replay repeat count: ", group.RepeatCount)
	replayArgs, err := json.Marshal(*group)
	if err != nil {
		return errors.Wrap(err, "failed to marshal TestGroupConfig")
	}

	// defaultSwappinessValue is used to keep record of the default swappiness of the DUT,
	// it will be used to recover the default setting after the replays are done.
	var defaultSwappinessValue uint32
	if group.Swappiness > 0 {
		swappinessValue, err := readHostSwappiness()
		if err != nil {
			return errors.Wrap(err, "failed to get swappiness value")
		}
		defaultSwappinessValue = swappinessValue
		testing.ContextLog(ctx, "Default swappiness value is: ", defaultSwappinessValue)
		// modify the host's swappiness to the given value.
		errSetup := setupSwappiness(ctx, group.Swappiness)
		if errSetup != nil {
			return errors.Wrap(errSetup, "failed to setup swappiness value")
		}
	}

	// Run the trace replay in Guest through the cmd, which is implemented in graphics-utils-go.
	testing.ContextLog(ctx, "Running extended replay with args: "+string(replayArgs))
	replayOutput, err := guest.Command(ctx, path.Join(guest.GetBinPath(), replayAppName), string(replayArgs)).Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed while running trace_replay")
	}

	testing.ContextLog(ctx, "Extended Replay output: "+string(replayOutput))

	var testResult comm.TestGroupResult
	if err := json.Unmarshal(replayOutput, &testResult); err != nil {
		return errors.Wrapf(err, "unable to parse test group result output: %q", string(replayOutput))
	}

	if testResult.Result != comm.TestResultSuccess {
		return errors.Errorf("%s", testResult.Message)
	}
	// Generate Avg FPS over multiple runs.
	failedEntries := 0
	perfValues := perf.NewValues()
	// An sample of the replayOutput: https://paste.googleplex.com/6164898517090304
	for _, resultEntry := range testResult.Entries {
		if resultEntry.Message != "" {
			testing.ContextLog(ctx, resultEntry.Message)
		}
		if resultEntry.Result != comm.TestResultSuccess {
			failedEntries++
			continue
		}
		var totalFPS float64
		runs := 0
		for key, value := range resultEntry.Values {
			// Get rid of the 1st run
			if strings.Contains(key, "replay001") {
				continue
			}
			if strings.Contains(key, "fps") {
				runs++
				totalFPS += float64(value.Value)
			}
		}
		perfValues.Set(perf.Metric{
			Name:      resultEntry.Name + "_swappiness_" + strconv.FormatUint(uint64(group.Swappiness), 10),
			Unit:      "fps",
			Direction: perf.BiggerIsBetter,
		}, totalFPS/float64(runs))
	}
	if err := perfValues.Save(resultDir); err != nil {
		return errors.Wrap(err, "unable to save performance values")
	}

	// Set swappiness value back to the DUT's default value
	if group.Swappiness > 0 {
		errSetup := setupSwappiness(ctx, defaultSwappinessValue)
		if errSetup != nil {
			return errors.Wrap(errSetup, "failed to recover the deafult swappiness value")
		}
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

	// Dump pre-run info from the guest
	for _, entry := range preRunGuestLogEntryList {
		testing.ContextLogf(ctx, "Saving %s to %s", entry.entryName, entry.logFileName)
		if err := logGuestCommand(ctx, guest, entry.command, filepath.Join(outDir, entry.logFileName)); err != nil {
			testing.ContextLog(ctx, "WARNING: Unable to get ", entry.entryName)
		}
	}

	testing.ContextLog(ctx, "Running params: repeat count: ", group.RepeatCount)

	if err := getSystemInfo(&group.Host); err != nil {
		return errors.Wrap(err, "failed to get system info")
	}

	var gpi *graphicsPowerInterface
	if group.ExtendedDuration > 0 {
		gpi = &graphicsPowerInterface{signalRunningFile: testVars.PowerTestVars.SignalRunningFile, signalCheckpointFile: testVars.PowerTestVars.SignalCheckpointFile}

	}

	serverAddr := fmt.Sprintf("%s:%d", outboundIP, fileServerPort)
	server, err := startFileServer(ctx, serverAddr, outDir, cloudStorage, &group.Repository, gpi)
	if err != nil {
		return errors.Wrap(err, "failed to start file server")
	}
	defer func() {
		if err := server.Shutdown(ctx); err != nil {
			testing.ContextLog(ctx, "WARNING: Unable to shutdown file server: ", err)
		}
	}()

	if err := pushTraceReplayApp(ctx, guest, outboundIP, fileServerPort); err != nil {
		return errors.Wrap(err, "failed to push trace_replay to destination")
	}

	// Validate the protocol version of the guest trace_replay app
	replayAppVersionOutput, err := guest.Command(ctx, path.Join(guest.GetBinPath(), replayAppName), "--version").Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to get trace_replay version")
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
		testing.ContextLog(ctx, "running extended mode")
		return runTraceReplayExtendedInVM(shortCtx, resultDir, guest, group)
	}
	if group.RepeatCount > 0 {
		testing.ContextLog(ctx, "running tuning mode")
		return runTraceReplayTuningInVM(shortCtx, resultDir, guest, group)
	}
	testing.ContextLog(ctx, "running single mode")
	err = runTraceReplayInVM(shortCtx, resultDir, guest, group)

	// Dump logs from the guest
	for _, entry := range postRunGuestLogEntryList {
		testing.ContextLogf(ctx, "Saving %s to %s", entry.entryName, entry.logFileName)
		if err := logGuestCommand(ctx, guest, entry.command, filepath.Join(outDir, entry.logFileName)); err != nil {
			testing.ContextLog(ctx, "WARNING: Unable to get ", entry.entryName)
		}
	}

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
	if err != nil {
		return errors.Wrap(err, "failed to run trace_replay app in the VM")
	}
	return nil
}
