package dbus

import (
	"bytes"
	"context"
	"io"
	"sync"

	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

// buffer is the buffer object for collected logs.
type buffer struct {
	lock sync.Mutex
	buf  bytes.Buffer
}

// Write writes d into the buffer.
func (b *buffer) Write(d []byte) (int, error) {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.buf.Write(d)
}

// Dump copies the buffer to w and resets the buffer.
func (b *buffer) Dump(w io.Writer) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	if _, err := b.buf.WriteTo(w); err != nil {
		return err
	}
	b.buf.Reset()
	return nil
}

// Monitor runs and collects the output of dbus-monitor.
type Monitor struct {
	host *ssh.Conn
	buf  buffer
	cmd  *ssh.Cmd
}

// StartMonitor starts a new Monitor on the host.
func StartMonitor(ctx context.Context, host *ssh.Conn, dbusMonitorArgs ...string) (*Monitor, error) {
	c := &Monitor{
		host: host,
	}
	if err := c.start(ctx, dbusMonitorArgs); err != nil {
		return nil, err
	}
	return c, nil
}

// start spawns the dbus-monitor process to begin collecting D-Bus messages on
// the host as they are reported.
func (m *Monitor) start(ctx context.Context, dbusMonitorArgs []string) error {
	testing.ContextLogf(ctx, "Starting dbus-monitor with args %v", dbusMonitorArgs)
	cmd := m.host.CommandContext(ctx, "dbus-monitor", dbusMonitorArgs...)
	cmd.Stdout = &m.buf
	cmd.Stderr = &m.buf
	if err := cmd.Start(); err != nil {
		return errors.Wrapf(err, "failed to run dbus-monitor with args %v", dbusMonitorArgs)
	}
	m.cmd = cmd
	return nil
}

// Dump copies the contents collected to w and resets the buffer.
func (m *Monitor) Dump(w io.Writer) error {
	return m.buf.Dump(w)
}

// Close stops the collector.
func (m *Monitor) Close() error {
	m.cmd.Abort()
	// Ignore the error as wait always has error on aborted command.
	_ = m.cmd.Wait()
	return nil
}
