// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

// HistogramDiffer stores bucket values in chrome://histogram when it is created.
// Calculate the histogram's difference between the values and bucket values when End() is called.
type HistogramDiffer struct {
	// chrome is Chrome that opens chrome://histogram queried by HistogramDiffer.
	chrome *chrome.Chrome
	// histogramName is a histogram name to be read with this HistogramDiffer (e.g Media.GpuVideoDecoderError).
	histogramName string
	// The bucket values in histogram name when NewHistogramDiffer() is called.
	beginHistogram map[int]int
	// The bucket values in histogram name when End() is called.
	endHistogram map[int]int
}

// NewHistogramDiffer creates and returns a new HistogramDiffer.
// In this function, the bucket values in chrome://histogramName at that time are stored.
// cr is chrome histogramDiffer stores and will use.
// histogramName is a histogram name whose bucket values histogramDiffer reads.
func NewHistogramDiffer(s *testing.State, ctx context.Context, cr *chrome.Chrome, histogramName string) (*HistogramDiffer, error) {
	var err error
	hd := new(HistogramDiffer)
	hd.chrome = cr
	hd.histogramName = histogramName
	hd.beginHistogram, err = getHistogram(s, ctx, cr, histogramName)
	if err != nil {
		s.Error("Fail in getHistogram(): ", err)
		return nil, err
	}
	testing.ContextLogf(ctx, "begin histograms/%s: %v", hd.histogramName, hd.beginHistogram)
	return hd, nil
}

// End() returns the difference between the current bucket values and ones stored in NewHistogramDiffer().
// This function can be called multiple times.
func (hd *HistogramDiffer) End(s *testing.State, ctx context.Context) (map[int]int, error) {
	var err error
	hd.endHistogram, err = getHistogram(s, ctx, hd.chrome, hd.histogramName)
	if err != nil {
		s.Error("Fail in getHistogram(): ", err)
		return nil, err
	}
	testing.ContextLogf(ctx, "end histograms/%s: %v", hd.histogramName, hd.endHistogram)
	diff := subtractHistogram(hd.endHistogram, hd.beginHistogram)
	testing.ContextLogf(ctx, "diff histograms/%s: %v", hd.histogramName, diff)
	return diff, nil
}

// PollHistogramGrow calls HistogramDiffer::End() until End() returns non empty map.
// In other words, it tries to read bucket values until some difference from the beginning is caused.
// This returns histogram difference returned by End(). If no difference happens in timeOut, returns nil.
// sleepInterval is the interval before calling next End(). Unit is second.
// Unit of timeOut and sleepInterval are second.
func PollHistogramGrow(s *testing.State, ctx context.Context, hd *HistogramDiffer, timeOut float64, sleepInterval float64) map[int]int {
	tryTimes := int(timeOut / sleepInterval)
	var sleepSec time.Duration = time.Duration(sleepInterval * float64(time.Second))
	for i := 0; i < tryTimes; i++ {
		time.Sleep(sleepSec)
		diff, _ := hd.End(s, ctx)
		if diff != nil && len(diff) > 0 {
			return diff
		}
	}
	return nil
}

// subtractHistogram subtracts histogram: minuend - subtrahend
func subtractHistogram(minuend, subtrahend map[int]int) map[int]int {
	r := make(map[int]int)
	for k, v := range minuend {
		r[k] = v
	}
	for k, v := range subtrahend {
		r[k] -= v
	}
	r2 := make(map[int]int)
	for k, v := range r {
		if v != 0 {
			r2[k] = v
		}
	}
	return r2
}

// getHistogram opens chrome://histogram/histogramName and returns read bucket values as map[int]int.
func getHistogram(s *testing.State, ctx context.Context, cr *chrome.Chrome, histogramName string) (map[int]int, error) {
	// "chrome://about" cannot be opened directly, due to cdp
	// implementation.
	// https://github.com/mafredri/cdp/blob/master/devtool/devtool.go#L80
	// This is workaround; create the connection and then naviagate
	// to the page.
	conn, err := cr.NewConn(ctx, "")
	if err != nil {
		s.Error("Fail to create new Conn: ", err)
		return nil, err
	}
	defer conn.Close()
	if err = conn.Navigate(ctx, "chrome://histograms/"+histogramName); err != nil {
		s.Errorf("Fail to open chrome://histograms/%s: %v", histogramName, err)
		return nil, err
	}
	rawText := ""
	if err = conn.Eval(ctx, "document.documentElement && document.documentElement.innerText", &rawText); err != nil {
		s.Errorf("Fail to load text in chrome://histograms/%s: %v", histogramName, err)
		return nil, err
	}
	histogramText := ""
	if searchIndex := strings.Index(rawText, "Histogram:"); searchIndex != 1 {
		histogramText = strings.TrimSpace(rawText[searchIndex+len("Histogram:"):])
		testing.ContextLogf(ctx, "chrome://histograms/%s:\n%s", histogramName, histogramText)
	} else {
		s.Errorf("No histogram is shown in chrome://histograms/%s", histogramName)
		return nil, errors.New("No histgram text")
	}
	return parseHistogramText(s, histogramText)
}

// parseHistogramText parses histogram text and builds map[int]int from the text.
func parseHistogramText(s *testing.State, histogramText string) (map[int]int, error) {
	// Match separator line, e.g. "1   ..."
	separatorRegExp := regexp.MustCompile(`\d+\s+\.\.\.`)
	// Match bucket line, e.g. "2  --O  (46 = 1.5%) {46.1%}"
	bucketRegExp := regexp.MustCompile(`(\d+)\s+\-*O\s+\((\d+) = (\d+\.\d+)%\).*`)

	result := make(map[int]int)
	for _, ln := range strings.Split(string(histogramText), "\n") {
		if separatorRegExp.MatchString(ln) {
			continue
		}
		if m := bucketRegExp.FindStringSubmatch(ln); m != nil {
			var err error
			i, j := 0, 0
			if i, err = strconv.Atoi(m[1]); err != nil {
				s.Error("Failed to convert integer %s: %v", m[1], err)
				return nil, err
			}
			if j, err = strconv.Atoi(m[2]); err != nil {
				s.Error("Failed to convert integer %s: %v", m[2], err)
				return nil, err
			}
			result[i] = j
		}
	}
	return result, nil
}
