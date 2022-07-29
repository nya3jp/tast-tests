package memoryuser

import (
	"bufio"
	"context"
	"io"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

type MemoryAllocationUnit struct {
	Id    int
	Cmd   *testexec.Cmd
	stdin io.WriteCloser
}

func (t *MemoryAllocationUnit) Run(ctx context.Context, cmd *testexec.Cmd) error {
	t.Cmd = cmd
	stdoutPipe, err := t.Cmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "failed to get allocator stdout")
	}
	stdinPipe, err := t.Cmd.StdinPipe()
	if err != nil {
		return errors.Wrap(err, "failed to get allocator stdin")
	}
	t.stdin = stdinPipe
	if err := t.Cmd.Start(); err != nil {
		return errors.Wrap(err, "failed to start the allocation process")
	}
	go func() {
		t.Cmd.Wait()
	}()
	// Make sure the output is as expected, and wait until we are done
	// allocating.
	stdout := bufio.NewReader(stdoutPipe)
	if statusString, err := stdout.ReadString('\n'); err != nil {
		return errors.Wrap(err, "failed to read status from the allocation unit")
	} else if !strings.HasPrefix(statusString, "allocating ") {
		return errors.Errorf("failed to read status line, exptected \"allocating ...\", got %q", statusString)
	}
	if doneString, err := stdout.ReadString('\n'); err != nil {
		return errors.Wrap(err, "failed to read done from the Memory allocation unit")
	} else if doneString != "done\n" {
		return errors.Errorf("failed to read done line, exptected \"done\\n\", got %q", doneString)
	}

	return nil
}

func (t *MemoryAllocationUnit) StillAlive() bool {
	if t.Cmd == nil {
		return false
	}

	return t.Cmd.ProcessState == nil

}

func (t *MemoryAllocationUnit) Close() error {
	if t.Cmd == nil {
		return nil
	}
	if err := t.Cmd.Kill(); err != nil {
		return errors.Wrap(err, "failed to kill the memory allocation unit")
	}
	if err := t.Cmd.Wait(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to wait for the memory allocation unit after kill")
	}
	return nil
}
func NewMemoryAllocationUnit(id int) *MemoryAllocationUnit {
	var cmd *testexec.Cmd
	return &MemoryAllocationUnit{id, cmd, nil}
}
