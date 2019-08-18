// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package gtest provides utilities to run gtest binary on Tast.
// TODO(hidehiko): Merge chromiumos/tast/local/chrome/bintest package into
// this.
package gtest

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

// ListTests returns a list of tests in the gtest executable.
func ListTests(ctx context.Context, exec string) ([]string, error) {
	output, err := testexec.CommandContext(ctx, exec, "--gtest_list_tests").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, err
	}
	return parseTestList(string(output)), nil
}

// parseTestList parses the output of --gtest_list_tests and returns the
// list of test names. The format of output should be:
//
// TestSuiteA.
//   TestCase1
//   TestCase2
// TestSuiteB.
//   TestCase3
//   TestCase4
//
// etc. The each returned test name is formatted into "TestSuite.TestCase".
func parseTestList(content string) []string {
	var suite string
	var result []string
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, " ") {
			// Test case name.
			testCase := strings.TrimSpace(line)
			paramDelimiterIndex := strings.Index(testCase, "  #")
			if paramDelimiterIndex != -1 {
				testCase = testCase[:paramDelimiterIndex]
			}
			result = append(result, fmt.Sprintf("%s%s", suite, testCase))
		} else {
			// Test suite name. Note: suite contains a trailing period.
			suite = strings.TrimSpace(line)
		}
	}
	return result
}

// GTest is a struct to run gtest binary.
type GTest struct {
	// exec is a path to the gtest executable.
	// If this is for ARC, the exec should be the path in guest.
	exec string

	// logfile is a path to the log file, which will contain stdout and
	// stderr of the gtest execution.
	// Note: Because the gtest log could be long, if this is not specified,
	// the log wouldn't be recorded to the current test log.
	logfile string

	// tempLogfile is true if the logfile should be created with
	// ioutil.TestFile instead of os.Create().
	tempLogfile bool

	// filter specifies a subset of tests to run. If not empty, the value
	// is passed to --gtest_filter=.
	// Please see the gtest manual for the specification.
	// Note that setting an empty string means no filtering.
	filter string

	// repeat specifies a number of repeating times to run the test.
	// The value is passed to --gtest_repeat=.
	// Note that "-1" means infinite.
	repeat int

	// extraArgs will be passed to the test execution. Note that all
	// --gtest* prefixed commandline flags should be constructed from
	// GTest struct internally, so it is an error to include --gtest* flags
	// in this.
	extraArgs []string

	// uid specifies the user to run.
	// -1 (by default) means unspecified, i.e., it runs as the current user
	// running this Tast test (practically, root).
	// If this is for ARC, uid should be the one in guest.
	uid int

	// a is ARC instance to run gtest executable in ARC.
	// This is nil when gtest runs on Chrome OS host side.
	a *arc.ARC
}

// option is a self-referential function can be used to configure GTest.
type option func(t *GTest)

// Logfile returns an option to set logfile path of GTest.
func Logfile(path string) option {
	return func(t *GTest) { t.logfile = path }
}

// TempLogfile returns an option to set logfile path of GTest. The file is
// created by using ioutil.TempFile to avoid conflict, so its special pattern
// is usable here.
func TempLogfile(path string) option {
	return func(t *GTest) {
		t.logfile = path
		t.tempLogfile = true
	}
}

// Filter returns an option to set gtest_filter.
func Filter(pattern string) option {
	return func(t *GTest) { t.filter = pattern }
}

// Repeat returns an option to set gtest_repeat.
func Repeat(repeat int) option {
	return func(t *GTest) { t.repeat = repeat }
}

// ExtraArgs returns an option to pass more arguments than gtest arguments
// for execution.
func ExtraArgs(args ...string) option {
	return func(t *GTest) { t.extraArgs = args }
}

// UID returns an option to specify the user to run the gtest.
func UID(uid int) option {
	return func(t *GTest) { t.uid = uid }
}

// ARC returns an option to run the test in ARC.
func ARC(a *arc.ARC) option {
	return func(t *GTest) { t.a = a }
}

// New creates GTest instance with given options.
func New(exec string, opts ...option) *GTest {
	ret := &GTest{exec: exec, uid: -1}
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

// Args returns an array of string for execution.
func (t *GTest) Args() ([]string, error) {
	args := []string{t.exec}
	if t.filter != "" {
		args = append(args, "--gtest_filter="+t.filter)
	}
	if t.repeat != 0 {
		args = append(args, "--gtest_repeat="+strconv.Itoa(t.repeat))
	}

	// Verify extraArgs and append them.
	var gtestArgs []string
	for _, a := range t.extraArgs {
		if strings.HasPrefix(a, "--gtest") {
			gtestArgs = append(gtestArgs, a)
		}
	}
	if len(gtestArgs) > 0 {
		return nil, errors.Errorf("gtest.ExtraArgs should not contain --gtest prefixed flags: %v", gtestArgs)
	}
	args = append(args, t.extraArgs...)

	// If user to run the test is given, run under sudo.
	if t.uid >= 0 {
		args = append([]string{"sudo", "--user=#" + strconv.Itoa(t.uid)}, args...)
	}

	return args, nil
}

// runner defines thin APIs to implement GTest.Run() and Start().
type runner interface {
	// mktemp creates a tempfile with the given name and some generated suffix.
	mktemp(ctx context.Context, name string) (string, error)

	// chown changes the owner of the file.
	chown(ctx context.Context, path string, uid int) error

	// remove removes the file at the path.
	remove(ctx context.Context, path string) error

	// command creates *testexec.Cmd instance to be executed.
	command(ctx context.Context, args []string) *testexec.Cmd

	// read reads the file at the path.
	read(ctx context.Context, path string) ([]byte, error)
}

// crosRunner is the implementation of the runner for gtest execution on Chrome OS host environment.
type crosRunner struct{}

func (*crosRunner) mktemp(ctx context.Context, name string) (path string, retErr error) {
	f, err := ioutil.TempFile("", name)
	if err != nil {
		return "", err
	}
	defer func() {
		if retErr != nil {
			os.Remove(f.Name())
		}
	}()

	if err := f.Close(); err != nil {
		return "", err
	}
	return filepath.Abs(f.Name())
}

func (*crosRunner) chown(ctx context.Context, path string, uid int) error {
	return os.Chown(path, uid, -1)
}

func (*crosRunner) remove(ctx context.Context, path string) error {
	return os.Remove(path)
}

func (*crosRunner) command(ctx context.Context, args []string) *testexec.Cmd {
	return testexec.CommandContext(ctx, args[0], args[1:]...)
}

func (*crosRunner) read(ctx context.Context, path string) ([]byte, error) {
	return ioutil.ReadFile(path)
}

// arcRunner is the implementation of the runner for gtest execution in ARC.
type arcRunner struct {
	a *arc.ARC
}

func (r *arcRunner) mktemp(ctx context.Context, name string) (string, error) {
	out, err := r.a.Command(ctx, "mktemp", "-p", arc.ARCTmpDirPath, name+".XXXXXX").Output(testexec.DumpLogOnError)
	return strings.TrimSpace(string(out)), err
}

func (r *arcRunner) chown(ctx context.Context, path string, uid int) error {
	return r.a.Command(ctx, "chown", strconv.Itoa(uid), path).Run(testexec.DumpLogOnError)
}

func (r *arcRunner) remove(ctx context.Context, path string) error {
	return r.a.Command(ctx, "rm", path).Run(testexec.DumpLogOnError)
}

func (r *arcRunner) command(ctx context.Context, args []string) *testexec.Cmd {
	return r.a.Command(ctx, args[0], args[1:]...)
}

func (r *arcRunner) read(ctx context.Context, path string) ([]byte, error) {
	return r.a.ReadFile(ctx, path)
}

func (t *GTest) runner() runner {
	if t.a == nil {
		return &crosRunner{}
	}
	return &arcRunner{a: t.a}
}

// Run executes the gtest, and returns the parsed Report.
// Note that the returned Report may not be nil even if test command execution
// returns an error. E.g., if test case in the gtest fails, the command will
// return an error, but the report file should be created. This function
// also handles the case, and returns it.
func (t *GTest) Run(ctx context.Context) (*Report, error) {
	r := t.runner()

	// Create a report file.
	output, err := createOutput(ctx, r, t.uid)
	if err != nil {
		return nil, err
	}
	defer r.remove(ctx, output)

	cmd, err := t.startCommand(ctx, r, output)
	if err != nil {
		return nil, err
	}
	retErr := cmd.Wait()

	// Parse output file regardless of whether the command succeeded or
	// not. Specifically, if a test case fail, the command reports an
	// error, but the report file should be properly created.
	content, err := r.read(ctx, output)
	if err != nil {
		if retErr == nil {
			return nil, err
		}
		testing.ContextLog(ctx, "Failed to read output log: ", err)
		return nil, retErr
	}
	report, err := parseReportInternal(content)
	if err != nil {
		if retErr == nil {
			retErr = err
		} else {
			// Just log the parse error if the command execution
			// already fails.
			testing.ContextLog(ctx, "Failed to parse report: ", err)
		}
	}

	return report, retErr
}

// createOutput creates the XML output file. If uid is given (i.e. not negative)
// the file is owned by the user, because gtest process opens the file directly.
// Returns the absolute path to the file.
func createOutput(ctx context.Context, r runner, uid int) (path string, err error) {
	p, err := r.mktemp(ctx, "gtest_output.xml")
	if err != nil {
		return "", errors.Wrap(err, "failed to create an output file")
	}
	defer func() {
		if err != nil {
			r.remove(ctx, p)
		}
	}()

	if uid >= 0 {
		if err := r.chown(ctx, p, uid); err != nil {
			return "", errors.Wrap(err, "failed to set user for the output file")
		}
	}

	return p, nil
}

// Start executes the gtest asynchronously, and returns the testexec.Cmd
// instance to talk to the process.
func (t *GTest) Start(ctx context.Context) (*testexec.Cmd, error) {
	return t.startCommand(ctx, t.runner(), "" /* output */)
}

func (t *GTest) startCommand(ctx context.Context, r runner, output string) (*testexec.Cmd, error) {
	args, err := t.Args()
	if err != nil {
		return nil, err
	}
	if output != "" {
		args = append(args, "--gtest_output=xml:"+output)
	}

	// Set up log output.
	var log *os.File
	if t.logfile != "" {
		var err error
		log, err = openLogfile(t.logfile, t.tempLogfile)
		if err != nil {
			return nil, err
		}
		// log needs to be closed after cmd starts.
		defer log.Close()

		// At the beginning of the log file, write the command line
		// to make debugging easier.
		if err := writeArgs(log, args); err != nil {
			return nil, err
		}
	}

	cmd := r.command(ctx, args)
	// Redirect stdout and stderr. Note that if logfile is not specified,
	// log is nil, which means redirecting to /dev/null.
	cmd.Stdout = log
	cmd.Stderr = log

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return cmd, nil
}

// openLogfile creates and opens the log file at path. If tempfile is set true,
// ioutil.TempFile is used. Specifically, some random string will be appended
// at the end of path, or last '*' is expanded to a random string. Please see
// also ioutil.TempFile's comment for details.
func openLogfile(path string, tempfile bool) (*os.File, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, errors.Wrap(err, "failed to create a log directory")
	}

	var f *os.File
	var err error
	if tempfile {
		f, err = ioutil.TempFile(dir, filepath.Base(path))
	} else {
		f, err = os.Create(path)
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a log file")
	}

	return f, nil
}

// writeArgs writes the given args into the file stream, and flushes it.
func writeArgs(f *os.File, args []string) error {
	if _, err := fmt.Fprintf(f, "Running %s\n\n", shutil.EscapeSlice(args)); err != nil {
		return errors.Wrap(err, "failed to write command line to a log file")
	}

	// Then flush, so that the stdout/stderr redirected from gtest executable
	// should follow.
	if err := f.Sync(); err != nil {
		return errors.Wrap(err, "failed to flush log file")
	}

	return nil
}
