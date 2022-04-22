// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	gotesting "testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
	"chromiumos/tast/testutil"
)

// crashFile contains information about a crash file used by tests.
// The testutil package uses relative paths while the crash package
// uses absolute paths, so this struct stores both.
type crashFile struct{ rel, abs, data string }

// writeCrashFile writes a file with relative path rel containing data to dir.
func writeCrashFile(t *gotesting.T, dir, rel, data string) crashFile {
	cf := crashFile{rel, filepath.Join(dir, rel), data}
	if err := testutil.WriteFiles(dir, map[string]string{rel: data}); err != nil {
		t.Fatal(err)
	}
	return cf
}

func TestGetCrashes(t *gotesting.T) {
	td := testutil.TempDir(t)
	defer os.RemoveAll(td)

	writeCrashFile(t, td, "foo.txt", "") // skipped because non-core/dmp extension
	fooCore := writeCrashFile(t, td, "foo.core", "")
	fooDmp := writeCrashFile(t, td, "foo.dmp", "")
	fooLog := writeCrashFile(t, td, "foo.log", "")
	fooMeta := writeCrashFile(t, td, "foo.meta", "")
	fooInfo := writeCrashFile(t, td, "foo.info", "")
	fooProclog := writeCrashFile(t, td, "foo.proclog", "")
	fooGPU := writeCrashFile(t, td, "foo.i915_error_state.log.xz", "")
	fooCompressedTxt := writeCrashFile(t, td, "foo.txt.gz", "")
	fooBIOSLog := writeCrashFile(t, td, "foo.bios_log", "")
	fooKCrash := writeCrashFile(t, td, "foo.kcrash", "")
	fooCompressedLog := writeCrashFile(t, td, "foo.log.gz", "")
	barDmp := writeCrashFile(t, td, "bar.dmp", "")
	writeCrashFile(t, td, "bar", "")            // skipped because no extenison
	writeCrashFile(t, td, "subdir/baz.dmp", "") // skipped because in subdir
	writeCrashFile(t, td, "foo.info.gz", "")    // skipped because second extension is wrong
	writeCrashFile(t, td, "other.xz", "")

	dirs := []string{filepath.Join(td, "missing"), td} // nonexistent dir should be skipped
	files, err := GetCrashes(dirs...)
	if err != nil {
		t.Fatalf("GetCrashes(%v) failed: %v", dirs, err)
	}
	sort.Strings(files)
	if exp := []string{barDmp.abs, fooBIOSLog.abs, fooCore.abs, fooDmp.abs, fooGPU.abs, fooInfo.abs, fooKCrash.abs, fooLog.abs, fooCompressedLog.abs, fooMeta.abs, fooProclog.abs, fooCompressedTxt.abs}; !reflect.DeepEqual(files, exp) {
		t.Errorf("GetCrashes(%v) = %v; want %v", dirs, files, exp)
	}
}

func TestProcessRunning(t *gotesting.T) {
	td := testutil.TempDir(t)
	defer os.RemoveAll(td)

	// procName must be <=14 characters long so that gopsutil doesn't look at
	// /proc/$$/cmdline.
	procName := fmt.Sprintf("t_%d", rand.Int31())
	if err := ioutil.WriteFile(filepath.Join(td, procName), []byte("#!/bin/sh\nsleep 10\n"), 0777); err != nil {
		t.Fatal("Failed to write a script: ", err)
	}

	cmd := exec.Command(filepath.Join(td, procName))
	if err := cmd.Start(); err != nil {
		t.Fatal("Failed to start a script: ", err)
	}
	func() {
		defer cmd.Wait()
		defer cmd.Process.Kill()

		if err := testing.Poll(context.Background(), func(ctx context.Context) error {
			running, err := processRunning(procName)
			if err != nil {
				return testing.PollBreak(err)
			}
			if !running {
				return errors.New("processRunning = false")
			}
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			t.Fatal("Failed to wait for processRunning to return true: ", err)
		}
	}()

	if err := testing.Poll(context.Background(), func(ctx context.Context) error {
		running, err := processRunning(procName)
		if err != nil {
			return testing.PollBreak(err)
		}
		if running {
			return errors.New("processRunning = true")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		t.Fatal("Failed to wait for processRunning to return false: ", err)
	}
}

func compareWaitForCrashFilesResultHelper(results, expected map[string][]string, td, message string) error {
	// Order doesn't matter for the list of files, so sort before comparison.
	for _, value := range expected {
		for i := range value {
			value[i] = filepath.Join(td, value[i])
		}
		sort.Strings(value)
	}
	for _, value := range results {
		sort.Strings(value)
	}
	if !reflect.DeepEqual(results, expected) {
		return errors.Errorf(message, results, expected)
	}

	return nil
}

func compareWaitForCrashFilesResult(results, expected map[string][]string, td string) error {
	return compareWaitForCrashFilesResultHelper(results, expected, td, "WaitForCrashFiles returned %v, expected %v")
}

func compareRegexesNotFoundPartialMatches(results, expected map[string][]string, td string) error {
	return compareWaitForCrashFilesResultHelper(results, expected, td, "RegexesNotFound.PartialMatches was %v, expected %v")
}

func compareRegexesNotFound(actualErr error, expectedErr *RegexesNotFound, td string) error {
	if actualErr == nil {
		return errors.Errorf("WaitForCrashFiles succeeded, expected RegexesNotFound error %v", expectedErr)
	}
	notFoundErr, ok := actualErr.(RegexesNotFound)
	if !ok {
		return errors.Errorf("WaitForCrashFiles returned error %v, expected RegexesNotFound error %v", actualErr, expectedErr)
	}
	var issues []string

	for i := range expectedErr.Files {
		expectedErr.Files[i] = filepath.Join(td, expectedErr.Files[i])
	}
	// Order doesn't matter for the list of files, so sort before comparison.
	sort.Strings(expectedErr.Files)
	sort.Strings(notFoundErr.Files)
	if !reflect.DeepEqual(notFoundErr.Files, expectedErr.Files) {
		issues = append(issues, fmt.Sprintf("Bad Files list in err, got %v, expected %v", notFoundErr.Files, expectedErr.Files))
	}

	// Order doesn't matter for the missing list, so sort before comparison.
	sort.Strings(expectedErr.Missing)
	sort.Strings(notFoundErr.Missing)
	if !reflect.DeepEqual(notFoundErr.Missing, expectedErr.Missing) {
		issues = append(issues, fmt.Sprintf("Bad Missing list in err, got %v,  expected %v", notFoundErr.Missing, expectedErr.Missing))
	}

	if err := compareRegexesNotFoundPartialMatches(notFoundErr.PartialMatches, expectedErr.PartialMatches, td); err != nil {
		issues = append(issues, err.Error())
	}

	if len(issues) > 0 {
		return errors.Errorf("Returned RegexesNotFound error didn't match expected %v", issues)
	}

	return nil
}

func writeFiles(dir string, files []string, fileContents []byte, overrideFileContents map[string][]byte) error {
	for _, file := range files {
		contents := fileContents
		if val, ok := overrideFileContents[file]; ok {
			contents = val
		}
		if err := ioutil.WriteFile(filepath.Join(dir, file), contents, 0666); err != nil {
			return errors.Wrap(err, "failed to touch file")
		}
	}

	return nil
}

const (
	metaRegex   = `.*\.\d{1,8}\.meta`
	dmpRegex    = `.*\.\d{1,8}\.dmp`
	logRegex    = `.*\.\d{1,8}\.log`
	kcrashRegex = `.*\.\d{1,8}\.kcrash`
)

var waitForCrashFilesTests = []struct {
	// name is the test name.
	name string
	// files is the list of files to create before calling WaitForCrashFiles.
	files []string
	// laterFiles is a list of files that are created while WaitForCrashFiles is
	// running.
	laterFiles []string
	// fileContents is the contents of the files. (By default, all files have the
	// same contents). Usually contains 'done=1' so that meta files are considered
	// valid.
	fileContents []byte
	// overrideFileContents is a map of file name to contents. If present, it
	// overrides fileContents for the files that appear in the map.
	overrideFileContents map[string][]byte
	// dirs is the list of directories to pass into WaitForCrashFiles. The test
	// directory name is prepended to each. If empty or nil, the test directory
	// is passed in as the only directory.
	dirs []string
	// regexes is the regexes parameter to WaitForCrashFiles
	regexes []string
	// optionalRegexes, if not nil, is passed to WaitForCrashFiles via the
	// OptionalRegexes option.
	optionalRegexes []string
	// expectedResults is the expected non-error return value. The value strings
	// are the base names of the files, which will have the directory prepended
	// during the test.
	expectedResults map[string][]string
	// expectedErr, if not nil, is the RegexesNotFound error we expect to get from
	// WaitForCrashFiles. If nil, we don't expect to get an error at all.
	// Note that the various files (in Files and PartialMatches) will have the
	// directory prepended, so only the base names are listed in the test case.
	expectedErr *RegexesNotFound
	// timeout, if not 0, is the timeout for WaitForCrashFiles
	timeout time.Duration
}{{
	name: "Success",
	// "notreturned.*" will not be returned because it's missing the PID and so
	// doesn't match the pattern we provide below.
	files:        []string{"a.15.meta", "a.15.dmp", "a.15.log", "b.7.meta", "b.7.dmp", "b.7.log", "notreturned.meta", "notreturned.dmp", "notreturned.log"},
	fileContents: []byte("foo=bar\ndone=1"),
	regexes:      []string{metaRegex, dmpRegex, logRegex},
	expectedResults: map[string][]string{
		metaRegex: {"a.15.meta", "b.7.meta"},
		dmpRegex:  {"a.15.dmp", "b.7.dmp"},
		logRegex:  {"a.15.log", "b.7.log"},
	},
}, {
	name: "MissOneRegex",
	// No .log files, only .dmp and .meta.
	files:        []string{"a.15.meta", "a.15.dmp", "b.7.meta", "b.7.dmp", "notreturned.6.kcrash"},
	fileContents: []byte("foo=bar\ndone=1"),
	regexes:      []string{metaRegex, dmpRegex, logRegex},
	expectedErr: &RegexesNotFound{
		Missing: []string{logRegex},
		Files:   []string{"a.15.meta", "a.15.dmp", "b.7.meta", "b.7.dmp", "notreturned.6.kcrash"},
		PartialMatches: map[string][]string{
			metaRegex: {"a.15.meta", "b.7.meta"},
			dmpRegex:  {"a.15.dmp", "b.7.dmp"},
		},
	},
	timeout: time.Second,
}, {
	name:         "MissAllRegexes",
	files:        []string{"notreturned.kcrash"},
	fileContents: []byte("foo=bar\ndone=1"),
	regexes:      []string{metaRegex, dmpRegex, logRegex},
	expectedErr: &RegexesNotFound{
		Missing:        []string{metaRegex, dmpRegex, logRegex},
		Files:          []string{"notreturned.kcrash"},
		PartialMatches: map[string][]string{},
	},
	timeout: time.Second,
}, {
	name: "MatchesEntireFilename",
	// "abc.15.meta" is a partial match for the regex but is not returned because
	// we only accept full matches.
	files:        []string{"c.15.meta", "abc.15.meta"},
	fileContents: []byte("foo=bar\ndone=1"),
	regexes:      []string{`c\.\d{1,8}\.meta`},
	expectedResults: map[string][]string{
		`c\.\d{1,8}\.meta`: {"c.15.meta"},
	},
}, {
	name: "DirectoryNamesNotPartOfMatch",
	// The string "dir1/q.15.log" matches the regex but the file is not returned
	// because "q.15.log" does not match the regex.
	files:        []string{"dir/q.15.log", "dir/dir.15.log"},
	fileContents: []byte("foo=bar"),
	dirs:         []string{"dir"},
	regexes:      []string{`d.*\.\d{1,8}\.log`},
	expectedResults: map[string][]string{
		`d.*\.\d{1,8}\.log`: {"dir/dir.15.log"},
	},
}, {
	// Ensure WaitForCrashFiles actually waits for the files to show up and doesn't
	// exit if they are not there when the function is first called.
	name:         "ActuallyWaits",
	files:        []string{"a.15.log", "a.15.dmp", "b.7.log", "b.7.dmp"},
	laterFiles:   []string{"a.15.meta", "b.7.kcrash"},
	fileContents: []byte("foo=bar\ndone=1"),
	regexes:      []string{metaRegex, dmpRegex, logRegex, kcrashRegex},
	expectedResults: map[string][]string{
		metaRegex:   {"a.15.meta"},
		dmpRegex:    {"a.15.dmp", "b.7.dmp"},
		logRegex:    {"a.15.log", "b.7.log"},
		kcrashRegex: {"b.7.kcrash"},
	},
}, {
	name:         "MetasNeedDone",
	files:        []string{"a.15.meta", "a.15.dmp", "a.15.log", "b.7.meta", "b.7.dmp", "b.7.log"},
	fileContents: []byte("foo=bar"),
	regexes:      []string{metaRegex, dmpRegex, logRegex},
	expectedErr: &RegexesNotFound{
		Missing: []string{metaRegex},
		Files:   []string{"a.15.meta", "a.15.dmp", "a.15.log", "b.7.meta", "b.7.dmp", "b.7.log"},
		PartialMatches: map[string][]string{
			logRegex: {"a.15.log", "b.7.log"},
			dmpRegex: {"a.15.dmp", "b.7.dmp"},
		},
	},
	timeout: time.Second,
}, {
	name:         "NonMetasDontNeedDone",
	files:        []string{"a.15.dmp", "a.15.log", "b.7.dmp", "b.7.log"},
	fileContents: []byte("foo=bar"),
	regexes:      []string{dmpRegex, logRegex},
	expectedResults: map[string][]string{
		dmpRegex: {"a.15.dmp", "b.7.dmp"},
		logRegex: {"a.15.log", "b.7.log"},
	},
}, {
	name:                 "OnlyOneMetaNeedsDone_FirstMetaHasDone",
	files:                []string{"a.15.meta", "a.15.dmp", "a.15.log", "b.7.meta", "b.7.dmp", "b.7.log"},
	fileContents:         []byte("foo=bar"),
	overrideFileContents: map[string][]byte{"a.15.meta": []byte("done=1")},
	regexes:              []string{metaRegex, dmpRegex, logRegex},
	expectedResults: map[string][]string{
		dmpRegex:  {"a.15.dmp", "b.7.dmp"},
		logRegex:  {"a.15.log", "b.7.log"},
		metaRegex: {"a.15.meta"}, // Note no b.7.meta
	},
}, {
	name:                 "OnlyOneMetaNeedsDone_SecondMetaHasDone",
	files:                []string{"a.15.meta", "a.15.dmp", "a.15.log", "b.7.meta", "b.7.dmp", "b.7.log"},
	fileContents:         []byte("foo=bar"),
	overrideFileContents: map[string][]byte{"b.7.meta": []byte("done=1")},
	regexes:              []string{metaRegex, dmpRegex, logRegex},
	expectedResults: map[string][]string{
		dmpRegex:  {"a.15.dmp", "b.7.dmp"},
		logRegex:  {"a.15.log", "b.7.log"},
		metaRegex: {"b.7.meta"}, // Note no a.15.meta
	},
}, {
	name: "MultipleDirectories",
	files: []string{"dir2/a.15.meta", "dir2/a.15.dmp", "dir2/a.15.log",
		"dir3/b.7.meta", "dir3/b.7.dmp", "dir3/b.7.log"},
	fileContents: []byte("foo=bar\ndone=1"),
	dirs:         []string{"dir1", "dir2", "dir3"},
	regexes:      []string{metaRegex, dmpRegex, logRegex},
	expectedResults: map[string][]string{
		dmpRegex:  {"dir2/a.15.dmp", "dir3/b.7.dmp"},
		logRegex:  {"dir2/a.15.log", "dir3/b.7.log"},
		metaRegex: {"dir2/a.15.meta", "dir3/b.7.meta"},
	},
}, {
	name:         "NonCrashFilesAlwaysIgnored",
	files:        []string{"a.dmp", "a.kcrash", "a.notaknownextension"},
	fileContents: []byte("foo=bar"),
	regexes:      []string{".*"},
	expectedResults: map[string][]string{
		".*": {"a.dmp", "a.kcrash"},
	},
}, {
	name:         "NonCrashFilesAlwaysIgnoredNoMatches",
	files:        []string{"a.notaknownextension"},
	fileContents: []byte("foo=bar"),
	regexes:      []string{".*"},
	expectedErr: &RegexesNotFound{
		Missing:        []string{".*"},
		Files:          nil,
		PartialMatches: map[string][]string{},
	},
	timeout: time.Second,
}, {
	name:            "OptionalRegexesAreMatched",
	files:           []string{"a.15.meta", "a.15.dmp", "a.15.log", "a.15.kcrash", "b.7.meta", "b.7.dmp", "b.7.log", "notreturned.meta", "notreturned.dmp", "notreturned.log"},
	fileContents:    []byte("foo=bar\ndone=1"),
	regexes:         []string{metaRegex, dmpRegex},
	optionalRegexes: []string{logRegex, kcrashRegex},
	expectedResults: map[string][]string{
		metaRegex:   {"a.15.meta", "b.7.meta"},
		dmpRegex:    {"a.15.dmp", "b.7.dmp"},
		logRegex:    {"a.15.log", "b.7.log"},
		kcrashRegex: {"a.15.kcrash"},
	},
}, {
	name:            "OptionalRegexesDontCauseErrors",
	files:           []string{"a.15.meta", "a.15.dmp", "a.15.log", "b.7.meta", "b.7.dmp", "b.7.log", "notreturned.meta", "notreturned.dmp", "notreturned.log"},
	fileContents:    []byte("foo=bar\ndone=1"),
	regexes:         []string{metaRegex, dmpRegex},
	optionalRegexes: []string{logRegex, kcrashRegex},
	// kcrashRegex not matched
	expectedResults: map[string][]string{
		metaRegex: {"a.15.meta", "b.7.meta"},
		dmpRegex:  {"a.15.dmp", "b.7.dmp"},
		logRegex:  {"a.15.log", "b.7.log"},
	},
}, {
	name:            "OnlyOptionalRegexesAlwaysSucceeds",
	files:           nil,
	regexes:         nil,
	optionalRegexes: []string{metaRegex, dmpRegex, logRegex, kcrashRegex},
	expectedResults: map[string][]string{},
}}

func TestWaitForCrashFiles(t *gotesting.T) {
	t.Parallel()
	for _, testCase := range waitForCrashFilesTests {
		testCase := testCase // NOTE: https://github.com/golang/go/wiki/CommonMistakes#using-goroutines-on-loop-iterator-variables
		t.Run(testCase.name, func(t *gotesting.T) {
			t.Parallel() // Run subtests in parallel because some have timeouts
			td := testutil.TempDir(t)
			defer os.RemoveAll(td)

			dirs := testCase.dirs
			for i := range dirs {
				dirs[i] = filepath.Join(td, dirs[i])
				if err := os.MkdirAll(dirs[i], 0770); err != nil {
					t.Fatal("Failed to make subdirectory ", dirs[i], ": ", err)
				}
			}
			if len(dirs) == 0 {
				dirs = []string{td}
			}

			if err := writeFiles(td, testCase.files, testCase.fileContents, testCase.overrideFileContents); err != nil {
				t.Fatal("Failed to create files: ", err)
			}
			if testCase.laterFiles != nil {
				time.AfterFunc(time.Second, func() {
					if err := writeFiles(td, testCase.laterFiles, testCase.fileContents, testCase.overrideFileContents); err != nil {
						t.Fatal("Failed to create late-arriving files: ", err)
					}
				})
			}

			var opts []WaitForCrashFilesOpt
			if testCase.timeout != 0 {
				opts = append(opts, Timeout(testCase.timeout))
			}
			if testCase.optionalRegexes != nil {
				opts = append(opts, OptionalRegexes(testCase.optionalRegexes))
			}
			results, err := WaitForCrashFiles(context.Background(), dirs, testCase.regexes, opts...)
			if testCase.expectedResults == nil && results != nil {
				t.Error("WaitForCrashFiles returned ", results, ", expected no results")
			} else {
				if err := compareWaitForCrashFilesResult(results, testCase.expectedResults, td); err != nil {
					t.Error(err)
				}
			}

			if testCase.expectedErr != nil {
				if compareErr := compareRegexesNotFound(err, testCase.expectedErr, td); compareErr != nil {
					t.Error(compareErr)
				}
			} else if err != nil {
				t.Error("WaitForCrashFiles had error: ", err, ", expected success")
			}
		})
	}
}

func TestDeleteCoreDumps(t *gotesting.T) {
	td := testutil.TempDir(t)
	defer os.RemoveAll(td)

	dir1 := filepath.Join(td, "dir1") // dir1 is missing
	dir2 := filepath.Join(td, "dir2")
	if err := os.Mkdir(dir2, 0777); err != nil {
		t.Fatal("Failed to create dir: ", err)
	}

	initFiles := []string{"a.core", "a.txt", "b.core", "b.dmp", "b.jpg"}
	for _, fn := range initFiles {
		if err := ioutil.WriteFile(filepath.Join(dir2, fn), nil, 0666); err != nil {
			t.Fatal("Failed to touch file: ", err)
		}
	}

	// filesIn returns a list of files under dir.
	filesIn := func(dir string) []string {
		fis, err := ioutil.ReadDir(dir)
		if err != nil {
			t.Fatal("ReadDir failed: ", err)
		}
		var files []string
		for _, fi := range fis {
			files = append(files, fi.Name())
		}
		sort.Strings(files)
		return files
	}

	// First, test the behavior when crash_reporter is running.
	func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		reporterRunning := func() (bool, error) {
			// Cancel the context so that deleteCoreDumps returns after observing exactly once that
			// crash_reporter is running.
			cancel()
			return true, nil
		}

		if err := deleteCoreDumps(ctx, []string{dir1, dir2}, reporterRunning); err == nil {
			t.Error("deleteCoreDumps succeeded unexpectedly while crash_reporter is running")
		}

		got := filesIn(dir2)
		if diff := cmp.Diff(got, initFiles); diff != "" {
			t.Error("Files mismatch after failed deleteCoreDumps (-got +want):\n", diff)
		}
	}()

	// Second, test the behavior when crash_reporter is not running.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reporterRunning := func() (bool, error) {
		// Create a new core file. This should not be deleted.
		for _, fn := range []string{"c.core", "c.dmp"} {
			if err := ioutil.WriteFile(filepath.Join(dir2, fn), nil, 0666); err != nil {
				t.Fatal("Failed to touch file: ", err)
			}
		}
		return false, nil
	}

	if err := deleteCoreDumps(ctx, []string{dir1, dir2}, reporterRunning); err != nil {
		t.Error("deleteCoreDumps failed when crash_reporter is not running: ", err)
	}

	// a.core: Not deleted because a.dmp does not exist.
	// b.core: Deleted.
	// c.core: Not deleted because it was created after waiting for crash_reporter.
	got := filesIn(dir2)
	want := []string{"a.core", "a.txt", "b.dmp", "b.jpg", "c.core", "c.dmp"}
	if diff := cmp.Diff(got, want); diff != "" {
		t.Error("Files mismatch after successful deleteCoreDumps (-got +want):\n", diff)
	}
}

func TestCleanupDevcoredump(t *gotesting.T) {
	td := testutil.TempDir(t)
	defer os.RemoveAll(td)

	dirCD := filepath.Join(td, KernelDevCDDir)
	dirCD1 := filepath.Join(dirCD, "devcd1")
	if err := os.MkdirAll(dirCD1, 0777); err != nil {
		t.Fatal("Failed to create dir: ", err)
	}

	if err := os.WriteFile(filepath.Join(dirCD1, "data"), nil, 0666); err != nil {
		t.Fatal("Failed to touch file: ", err)
	}

	if _, err := os.Stat(filepath.Join(dirCD1, "data")); os.IsNotExist(err) {
		t.Fatal("Can not find data")
	}

	// cleanDevcoredump doesn't delete the devcoredump instance, instead writes '0' it.
	// the deletion is handled by a concurrent kernel thread
	if err := cleanupDevcoredump(context.Background(), 100*time.Millisecond, dirCD, kernelDevCDName); err != nil {
		t.Fatalf("Couldn't cleanup devcoredump data file %s", dirCD1)
	}

	// check if a '0' was written to the devcoredump instance
	data, err := os.ReadFile(filepath.Join(dirCD1, "data"))
	if err != nil {
		t.Fatalf("Devcoredump data file doesn't exist in %s/", dirCD1)
	}
	if len(data) == 0 || data[0] != '0' {
		t.Fatalf("cleanupDevcoredump failed to write '0' to %s/data", dirCD1)
	}
}
