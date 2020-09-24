// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memory

import (
	"context"
	"io/ioutil"
	"math"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
)

// Limit allows tests to determine if memory use is close to a limit without
// having to know the specific memory counters used.
type Limit interface {
	// Distance returns the amount of memory that can be allocated in bytes
	// before the limit is reached. If negative, abs(Distance()) bytes must be
	// freed to go below the limit.
	Distance(ctx context.Context) (int64, error)
	// AssertNotReached returns an error if the limit has been reached. Useful
	// for Polls to get information about which limit was exceeded, and by how
	// much.
	AssertNotReached(ctx context.Context) error
}

// AvailableLimit is a Limit for ChromeOS available memory.
type AvailableLimit struct {
	margin int64
}

// AvailableLimit conforms to Limit interface.
var _ Limit = (*AvailableLimit)(nil)

// readFirstInt64 reads the first integer from a file.
func readFirstInt64(f string) (int64, error) {
	// Files will always just be a single line, so it's OK to read everything.
	data, err := ioutil.ReadFile(f)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to read file %q", f)
	}
	firstString := strings.Split(strings.TrimSpace(string(data)), " ")[0]
	firstInt64, err := strconv.ParseInt(firstString, 10, 64)
	if err != nil {
		return 0, errors.Wrapf(err, "unable to convert %q to integer", data)
	}
	return firstInt64, nil
}

// Distance returns the difference between available memory and the critical
// margin, in bytes. Result will be negative if available memory is below the
// critical margin.
func (l *AvailableLimit) Distance(_ context.Context) (int64, error) {
	const availableMemorySysFile = "/sys/kernel/mm/chromeos-low_mem/available"
	availableMiB, err := readFirstInt64(availableMemorySysFile)
	if err != nil {
		return 0, errors.Wrap(err, "failed to read ChromeOS available memory")
	}
	return (availableMiB * MiB) - l.margin, nil
}

// AssertNotReached checks that available memory is above the margin.
func (l *AvailableLimit) AssertNotReached(ctx context.Context) error {
	distance, err := l.Distance(ctx)
	if err != nil {
		return err
	}
	if distance <= 0 {
		return errors.Errorf("available memory %d is less than margin %d", distance+l.margin, l.margin)
	}
	return nil
}

// CriticalMargin returns the value of ChromeOS available memory below which
// tabs are killed, in bytes.
func CriticalMargin() (int64, error) {
	const marginMemorySysFile = "/sys/kernel/mm/chromeos-low_mem/margin"
	criticalMarginMiB, err := readFirstInt64(marginMemorySysFile)
	if err != nil {
		return 0, errors.Wrap(err, "failed to read ChromeOS critical margin")
	}
	return criticalMarginMiB * MiB, nil
}

// NewAvailableLimit creates a Limit that measures how far away ChromeOS
// available memory is from a specified margin, in bytes.
func NewAvailableLimit(margin int64) (*AvailableLimit, error) {
	return &AvailableLimit{margin}, nil
}

// NewAvailableCriticalLimit creates a Limit that measures how far away ChromeOS
// is from the critical memory pressure margin. Unlike
// NewAvailableIsCriticalLimit above, this it intended to keep ChromeOS memory
// pressure below critical.
func NewAvailableCriticalLimit() (*AvailableLimit, error) {
	criticalMargin, err := CriticalMargin()
	if err != nil {
		return nil, err
	}
	return &AvailableLimit{
		margin: criticalMargin,
	}, nil
}

// PageReclaimLimit is a Limit that uses /proc/zoneinfo to allow tests to
// allocate enough memory to trigger page reclaim, but not so much memory that
// they OOM.
type PageReclaimLimit struct {
	largeZones map[string]bool
}

// PageReclaimLimit conforms to Limit interface.
var _ Limit = (*PageReclaimLimit)(nil)

// Distance returns the smallest distance from a zone's free counter to half-way
// between its min and low watermark. If <= 0, this means that page reclaim has
// started and we are at risk of the Linux OOM Killer waking up.
func (l *PageReclaimLimit) Distance(_ context.Context) (int64, error) {
	infos, err := ReadZoneInfo()
	if err != nil {
		return 0, errors.Wrap(err, "failed to read zone counters")
	}
	var minDistance int64 = math.MaxInt64
	for _, info := range infos {
		if _, ok := l.largeZones[info.Name]; !ok {
			// Zone is not a large zone.
			continue
		}
		distance := int64(info.Free) - int64((info.Low+info.Min)/2)
		if distance < minDistance {
			minDistance = distance
		}
	}
	if minDistance == math.MaxInt64 {
		return 0, errors.New("no large zones found")
	}
	return minDistance, nil
}

// AssertNotReached checks that no zone has its free pages counter below
// (min+low)/2.
func (l *PageReclaimLimit) AssertNotReached(_ context.Context) error {
	infos, err := ReadZoneInfo()
	if err != nil {
		return errors.Wrap(err, "failed to read zone counters")
	}
	for _, info := range infos {
		if _, ok := l.largeZones[info.Name]; !ok {
			// Zone is not a large zone.
			continue
		}
		distance := int64(info.Free) - int64((info.Low+info.Min)/2)
		if distance <= 0 {
			return errors.Errorf("zone %q free %d is less than (min+low)/2 (%d+%d)/2", info.Name, info.Free, info.Min, info.Low)
		}
	}
	return nil
}

// NewPageReclaimLimit creates a Limit that measures how far away ChromeOS is
// from any Linux memory zone's free pages being half-way between the min and
// low watermarks. The intent is to trigger page reclaim by being below the low
// watermark, but keep away from the low watermark to avoid invoking the Linux
// OOM killer.
func NewPageReclaimLimit() (*PageReclaimLimit, error) {
	infos, err := ReadZoneInfo()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read zone counters")
	}
	const largeZoneMinMin = 4 * MiB
	largeZones := make(map[string]bool)
	for _, info := range infos {
		if info.Min > largeZoneMinMin {
			largeZones[info.Name] = true
		}
	}
	if len(largeZones) == 0 {
		return nil, errors.New("no large zones found")
	}
	return &PageReclaimLimit{largeZones}, nil
}

// CmdLimit implements the Limit interface by forwarding Distance requests to
// a remote process.
type CmdLimit struct {
	cmdWithPipes
}

// CmdLimit conforms to Limit interface.
var _ Limit = (*CmdLimit)(nil)

// Distance sends "distance\n" to the running command, and returns the distance
// provided.
func (l *CmdLimit) Distance(ctx context.Context) (int64, error) {
	if err := l.writeLine("distance"); err != nil {
		return 0, errors.Wrap(err, "failed to write \"distance\"")
	}
	line, err := l.readLine()
	if err != nil {
		return 0, errors.Wrap(err, "failed to read distance")
	}
	distance, err := strconv.ParseInt(line, 10, 64)
	if err != nil {
		return 0, errors.Wrap(err, "failed to parse distance")
	}
	return distance, nil
}

// AssertNotReached just uses Distance, since we can't provide any more context.
func (l *CmdLimit) AssertNotReached(ctx context.Context) error {
	distance, err := l.Distance(ctx)
	if err != nil {
		return err
	}
	if distance <= 0 {
		return errors.Errorf("memory is %d bytes past limit", -distance)
	}
	return nil
}

// NewCmdLimit takes a testexec.Cmd, starts it, and hooks up stdin and stdout so
// that memory limit distance requests can be made.
func NewCmdLimit(cmd *testexec.Cmd) (*CmdLimit, error) {
	cmdWithPipes, err := newCmdWithPipes(cmd)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create limit cmd")
	}
	return &CmdLimit{cmdWithPipes}, nil
}

// CompositeLimit combines a set of Limits.
type CompositeLimit struct {
	limits []Limit
}

// CompositeLimit conforms to Limit interface.
var _ Limit = (*CompositeLimit)(nil)

// Distance returns the minimum Distance returned by any child Limit.
func (l *CompositeLimit) Distance(ctx context.Context) (int64, error) {
	if len(l.limits) == 0 {
		return 0, errors.New("empty compositeLimit")
	}
	minDistance, err := l.limits[0].Distance(ctx)
	if err != nil {
		return 0, err
	}
	for i := 1; i < len(l.limits); i++ {
		distance, err := l.limits[i].Distance(ctx)
		if err != nil {
			return 0, err
		}
		if distance < minDistance {
			minDistance = distance
		}
	}
	return minDistance, nil
}

// AssertNotReached checks that child Limits are above their limits.
func (l *CompositeLimit) AssertNotReached(ctx context.Context) error {
	if len(l.limits) == 0 {
		return errors.New("empty compositeLimit")
	}
	for i := 1; i < len(l.limits); i++ {
		if err := l.limits[i].AssertNotReached(ctx); err != nil {
			return err
		}
	}
	return nil
}

// NewCompositeLimit creates a Limit that returns the minimum Distance()
// returned by all the passed limits.
func NewCompositeLimit(limits ...Limit) *CompositeLimit {
	return &CompositeLimit{limits}
}
