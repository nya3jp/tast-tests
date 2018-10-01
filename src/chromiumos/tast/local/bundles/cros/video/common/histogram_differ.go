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

type HistogramDiffer struct {
	chrome         *chrome.Chrome
	histogramName  string
	beginHistogram map[int]int
	endHistogram   map[int]int
}

func NewHistogramDiffer(ctx context.Context, cr *chrome.Chrome, histogramName string) (HistogramDiffer, error) {
	var err error
	hd := HistogramDiffer{}
	hd.chrome = cr
	hd.histogramName = histogramName
	hd.beginHistogram, err = getHistogram(ctx, cr, histogramName)
	if err != nil {
		return hd, err
	}
	testing.ContextLogf(ctx, "begin histograms/%s: %v", hd.histogramName, hd.beginHistogram)
	return hd, nil
}

func (hd *HistogramDiffer) End(ctx context.Context) (map[int]int, error) {
	var err error
	hd.endHistogram, err = getHistogram(ctx, hd.chrome, hd.histogramName)
	if err != nil {
		return nil, err
	}
	testing.ContextLogf(ctx, "end histograms/%s: %v", hd.histogramName, hd.endHistogram)
	diff := subtractHistogram(hd.endHistogram, hd.beginHistogram)
	testing.ContextLogf(ctx, "begin histograms/%s: %v", hd.histogramName, diff)
	return diff, nil
}

func PollHistogramGrow(ctx context.Context, hd HistogramDiffer, timeOut float64, sleepInterval float64) map[int]int {
	tryTimes := int(timeOut / sleepInterval)
	var sleepSec time.Duration = time.Duration(sleepInterval * float64(time.Second))
	for i := 0; i < tryTimes; i++ {
		time.Sleep(sleepSec)
		diff, _ := hd.End(ctx)
		if diff != nil {
			return diff
		}
	}
	return nil
}

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

func getHistogram(ctx context.Context, cr *chrome.Chrome, histogramName string) (map[int]int, error) {
	conn, err := cr.NewConn(ctx, "http://chrome://histograms/"+histogramName)
	if err != nil {
		testing.ContextLogf(ctx, "AKAN: %v", err)
		return nil, err
	}
	defer conn.Close()
	rawText := ""
	if err = conn.Eval(ctx, "document.documentElement && document.documentElement.innerText", &rawText); err != nil {
		testing.ContextLogf(ctx, "AKAN2")
		return nil, err
	}
	histogramText := ""
	if searchIndex := strings.Index(rawText, "Histogram:"); searchIndex != 1 {
		histogramText = strings.TrimSpace(rawText[searchIndex+len("Histogram:"):])
		testing.ContextLogf(ctx, "chrome://histograms/%s:\n%s", histogramName, histogramText)
	} else {
		testing.ContextLogf(ctx, "No histogram is shown in chrome://histograms/%s", histogramName)
		return nil, errors.New("No histgram text")
	}
	return parseHistogramText(ctx, histogramText)
}

func parseHistogramText(ctx context.Context, histogramText string) (map[int]int, error) {
	// Match separator line, e.g. "1   ..."
	separatorRegExp := regexp.MustCompile(`\d+\s+\.\.\.`)
	// Match bucket line, e.g. "2  --O  (46 = 1.5%) {46.1%}"
	bucketRegExp := regexp.MustCompile(`(\d+)\s+\-*O\s+\((\d+) = (\d+\.\d+)%\).*"`)

	var result map[int]int
	for _, ln := range strings.Split(string(histogramText), "\n") {
		if separatorRegExp.MatchString(ln) {
			continue
		}
		if m := bucketRegExp.FindStringSubmatch(ln); m != nil {
			var err error
			i, j := 0, 0
			if i, err = strconv.Atoi(m[0]); err != nil {
				testing.ContextLogf(ctx, "Failed to convert integer; %s", m[0])
				return nil, err
			}
			if j, err = strconv.Atoi(m[1]); err != nil {
				testing.ContextLogf(ctx, "Failed to convert integer; %s", m[1])
				return nil, err
			}
			result[i] = j
		}
	}
	return result, nil
}
