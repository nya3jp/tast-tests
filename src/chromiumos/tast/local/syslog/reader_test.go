// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package syslog

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"chromiumos/tast/errors"
	"chromiumos/tast/testutil"
)

const (
	fakeLine1             = "2019-12-10T11:17:28.123456+09:00 INFO foo[1234]: hello\n"
	fakeLine2             = "2019-12-10T11:17:29.123456+09:00 WARN bar[2345]: crashy\n"
	fakeLine3             = "2019-12-10T11:17:30.123456+09:00 INFO foo[1234]: world\n"
	chromeFakeLine1       = "[9346:9346:1212/160319.316821:VERBOSE1:tablet_mode_controller.cc(536)] lid\n"
	chromeFakeLine2       = "[9419:1:1212/160319.355476:VERBOSE1:breakpad_linux.cc(2079)] enabled\n"
	chromeFakeLine3       = "[24195:24208:1213/162938.602368:ERROR:drm_gpu_display_manager.cc(211)] ID 21692109949126656\n"
	chromeSyslogFakeLine1 = "2019-12-12T16:03:19.316821Z VERBOSE1 chrome[9346:9346]: [tablet_mode_controller.cc(536)] lid\n"
	chromeSyslogFakeLine2 = "2019-12-12T16:03:19.355476Z VERBOSE1 chrome[9419:1]: [breakpad_linux.cc(2079)] enabled\n"
	chromeSyslogFakeLine3 = "2019-12-13T16:29:38.602368Z ERROR chrome[24195:24208]: [drm_gpu_display_manager.cc(211)] ID 21692109949126656\n"
)

var (
	jst = time.FixedZone("JST", 9*60*60)

	fakeEntry1 = &Entry{
		Timestamp: time.Date(2019, 12, 10, 11, 17, 28, 123456000, jst),
		Severity:  "INFO",
		Tag:       "foo[1234]",
		Program:   "foo",
		PID:       1234,
		Content:   "hello",
		Line:      fakeLine1,
	}
	fakeEntry2 = &Entry{
		Timestamp: time.Date(2019, 12, 10, 11, 17, 29, 123456000, jst),
		Severity:  "WARN",
		Tag:       "bar[2345]",
		Program:   "bar",
		PID:       2345,
		Content:   "crashy",
		Line:      fakeLine2,
	}
	fakeEntry3 = &Entry{
		Timestamp: time.Date(2019, 12, 10, 11, 17, 30, 123456000, jst),
		Severity:  "INFO",
		Tag:       "foo[1234]",
		Program:   "foo",
		PID:       1234,
		Content:   "world",
		Line:      fakeLine3,
	}
)

func TestReaderRead(t *testing.T) {
	for _, tc := range []struct {
		name   string
		opts   []Option
		init   string
		writes []string
		want   []*Entry
	}{
		// Initial content handling tests:
		{
			name:   "InitEmpty",
			writes: []string{fakeLine1 + fakeLine2 + fakeLine3},
			want:   []*Entry{fakeEntry1, fakeEntry2, fakeEntry3},
		},
		{
			name:   "InitFullLine",
			init:   "this is the last log message\n",
			writes: []string{fakeLine1 + fakeLine2 + fakeLine3},
			want:   []*Entry{fakeEntry1, fakeEntry2, fakeEntry3},
		},
		{
			name:   "InitMiddleLine",
			init:   "this is ",
			writes: []string{"the last log message\n" + fakeLine1 + fakeLine2 + fakeLine3},
			want:   []*Entry{fakeEntry1, fakeEntry2, fakeEntry3},
		},
		// Write handling tests:
		{
			name:   "WriteAligned",
			writes: []string{fakeLine1, fakeLine2, fakeLine3},
			want:   []*Entry{fakeEntry1, fakeEntry2, fakeEntry3},
		},
		{
			name:   "WriteUnaligned",
			writes: []string{fakeLine1[:10], fakeLine1[10:] + fakeLine2 + fakeLine3[:20], fakeLine3[20:]},
			want:   []*Entry{fakeEntry1, fakeEntry2, fakeEntry3},
		},
		{
			name:   "WriteIncomplete",
			writes: []string{fakeLine1 + fakeLine2 + fakeLine3[:len(fakeLine3)-1]}, // drop the last newline
			want:   []*Entry{fakeEntry1, fakeEntry2},
		},
		// Parsing tests:
		{
			name:   "ParseNoPID",
			writes: []string{"2019-12-10T11:17:28.123456+09:00 INFO kernel: oops\n"},
			want: []*Entry{{
				Timestamp: time.Date(2019, 12, 10, 11, 17, 28, 123456000, jst),
				Severity:  "INFO",
				Tag:       "kernel",
				Program:   "kernel",
				Content:   "oops",
				Line:      "2019-12-10T11:17:28.123456+09:00 INFO kernel: oops\n",
			}},
		},
		{
			name:   "ParseColonInProgram",
			writes: []string{"2019-12-10T11:17:28.123456+09:00 INFO foo:bar: hi\n"},
			want: []*Entry{{
				Timestamp: time.Date(2019, 12, 10, 11, 17, 28, 123456000, jst),
				Severity:  "INFO",
				Tag:       "foo:bar",
				Program:   "foo:bar",
				Content:   "hi",
				Line:      "2019-12-10T11:17:28.123456+09:00 INFO foo:bar: hi\n",
			}},
		},
		// Option tests:
		{
			name:   "OptionProgram",
			opts:   []Option{Program("foo")},
			writes: []string{fakeLine1 + fakeLine2 + fakeLine3},
			want:   []*Entry{fakeEntry1, fakeEntry3},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tf, err := ioutil.TempFile("", "")
			if err != nil {
				t.Fatal("TempFile failed: ", err)
			}
			defer tf.Close()

			tf.WriteString(tc.init)

			opts := append([]Option{SourcePath(tf.Name())}, tc.opts...)
			r, err := NewReader(context.Background(), opts...)
			if err != nil {
				t.Fatal("NewReader failed: ", err)
			}
			defer r.Close()

			var got []*Entry

			readAll := func() {
				for {
					e, err := r.Read()
					if err == io.EOF {
						break
					}
					if err != nil {
						t.Fatal("Read failed: ", err)
					}
					got = append(got, e)
				}
			}

			readAll()
			for _, s := range tc.writes {
				tf.WriteString(s)
				readAll()
			}

			if diff := cmp.Diff(got, tc.want); diff != "" {
				t.Errorf("Result unmatched (-got +want):\n%s", diff)
			}
		})
	}
}

func TestReaderParseFailure(t *testing.T) {
	tf, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal("TempFile failed: ", err)
	}
	defer tf.Close()

	r, err := NewReader(context.Background(), SourcePath(tf.Name()))
	if err != nil {
		t.Fatal("NewReader failed: ", err)
	}
	defer r.Close()

	const line = "abc\n"
	tf.WriteString(line)

	_, err = r.Read()
	if err == nil {
		t.Fatal("Read unexpectedly suceeded for a broken line")
	}
	perr, ok := err.(*ParseError)
	if !ok {
		t.Fatalf("Read returned an error of type %T; want *ParseError", err)
	}
	if perr.Line != line {
		t.Errorf("ParseError.Line = %q; want %q", perr.Line, line)
	}
}

func readAll(t *testing.T, r *Reader) []*Entry {
	t.Helper()
	var es []*Entry
	for {
		e, err := r.Read()
		if err == io.EOF {
			return es
		}
		if err != nil {
			t.Fatal("Read failed: ", err)
		}
		es = append(es, e)
	}
}

func TestReaderReadLogRotation(t *testing.T) {
	td := testutil.TempDir(t)
	defer os.RemoveAll(td)

	path := filepath.Join(td, "syslog")

	if err := ioutil.WriteFile(path, nil, 0644); err != nil {
		t.Fatal("Failed to create an empty syslog: ", err)
	}

	r, err := NewReader(context.Background(), SourcePath(path))
	if err != nil {
		t.Fatal("NewReader failed: ", err)
	}
	defer r.Close()

	if err := ioutil.WriteFile(path, []byte(fakeLine1), 0644); err != nil {
		t.Fatal("Failed to write the first entry: ", err)
	}
	if err := os.Rename(path, path+".rotated"); err != nil {
		t.Fatal("Rename failed: ", err)
	}
	if err := ioutil.WriteFile(path, []byte(fakeLine2), 0644); err != nil {
		t.Fatal("Failed to write the second entry: ", err)
	}

	got := readAll(t, r)
	want := []*Entry{fakeEntry1, fakeEntry2}
	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("Result unmatched (-got +want):\n%s", diff)
	}
}

func TestReaderReadLogRotationRace(t *testing.T) {
	td := testutil.TempDir(t)
	defer os.RemoveAll(td)

	path := filepath.Join(td, "syslog")

	if err := ioutil.WriteFile(path, nil, 0644); err != nil {
		t.Fatal("Failed to create an empty syslog: ", err)
	}

	r, err := NewReader(context.Background(), SourcePath(path))
	if err != nil {
		t.Fatal("NewReader failed: ", err)
	}
	defer r.Close()

	if err := ioutil.WriteFile(path, []byte(fakeLine1), 0644); err != nil {
		t.Fatal("Failed to write the first entry: ", err)
	}
	if err := os.Rename(path, path+".rotated"); err != nil {
		t.Fatal("Rename failed: ", err)
	}

	// Simulate the race condition where a log is rotated but the new
	// file isn't created yet.
	got := readAll(t, r)
	want := []*Entry{fakeEntry1}
	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("Result unmatched (-got +want):\n%s", diff)
	}

	if err := ioutil.WriteFile(path, []byte(fakeLine2), 0644); err != nil {
		t.Fatal("Failed to write the second entry: ", err)
	}

	got = readAll(t, r)
	want = []*Entry{fakeEntry2}
	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("Result unmatched (-got +want):\n%s", diff)
	}
}

func TestReaderWait(t *testing.T) {
	for _, tc := range []struct {
		name     string
		opts     []Option
		pred     EntryPred
		write    string
		want     *Entry
		wantNext *Entry
		wantErr  error
	}{
		{
			name:     "Found",
			pred:     func(e *Entry) bool { return e.Content == "crashy" },
			write:    fakeLine1 + fakeLine2 + fakeLine3,
			want:     fakeEntry2,
			wantNext: fakeEntry3,
		},
		{
			name:    "NotFound",
			pred:    func(e *Entry) bool { return false },
			write:   fakeLine1 + fakeLine2 + fakeLine3,
			wantErr: ErrNotFound,
		},
		{
			name: "FoundWithOptions",
			opts: []Option{Program("foo")},
			pred: func(e *Entry) bool { return e.Content == "world" },
			write: `2019-12-10T11:17:28.123456+09:00 INFO foo[1234]: hello
2019-12-10T11:17:29.123456+09:00 INFO bar[2345]: world
2019-12-10T11:17:30.123456+09:00 INFO foo[1234]: world
2019-12-10T11:17:31.123456+09:00 INFO bar[2345]: end
2019-12-10T11:17:32.123456+09:00 INFO foo[1234]: end
`,
			want: &Entry{
				Timestamp: time.Date(2019, 12, 10, 11, 17, 30, 123456000, jst),
				Severity:  "INFO",
				Tag:       "foo[1234]",
				Program:   "foo",
				PID:       1234,
				Content:   "world",
				Line:      "2019-12-10T11:17:30.123456+09:00 INFO foo[1234]: world\n",
			},
			wantNext: &Entry{
				Timestamp: time.Date(2019, 12, 10, 11, 17, 32, 123456000, jst),
				Severity:  "INFO",
				Tag:       "foo[1234]",
				Program:   "foo",
				PID:       1234,
				Content:   "end",
				Line:      "2019-12-10T11:17:32.123456+09:00 INFO foo[1234]: end\n",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tf, err := ioutil.TempFile("", "")
			if err != nil {
				t.Fatal("TempFile failed: ", err)
			}
			defer tf.Close()

			opts := append([]Option{SourcePath(tf.Name())}, tc.opts...)
			r, err := NewReader(context.Background(), opts...)
			if err != nil {
				t.Fatal("NewReader failed: ", err)
			}
			defer r.Close()

			tf.WriteString(tc.write)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			got, err := r.Wait(ctx, 1*time.Nanosecond, tc.pred)
			if err := ctx.Err(); err != nil {
				t.Fatal("Wait timed out: ", err)
			}
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("Wait failed with an unexpected error %q; want %q", err, tc.wantErr)
				}
			} else {
				if err != nil {
					t.Error("Wait failed: ", err)
				} else if diff := cmp.Diff(got, tc.want); diff != "" {
					t.Errorf("Wait returned an unexpected entry (-got +want):\n%s", diff)
				}
				got, err := r.Read()
				if err != nil {
					t.Error("Read failed: ", err)
				} else if diff := cmp.Diff(got, tc.wantNext); diff != "" {
					t.Errorf("Read returned an unexpected entry (-got +want):\n%s", diff)
				}
			}
		})
	}
}
