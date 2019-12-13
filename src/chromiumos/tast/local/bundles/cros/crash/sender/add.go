package sender

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/lsbrelease"
)

func AddFakeMinidumpCrash(ctx context.Context, dir, basename, exec, ver string) (expected *SendData, err error) {
	dmp := bytes.Repeat([]byte{0x28}, 1024*1024)
	if err := ioutil.WriteFile(filepath.Join(dir, basename+".dmp"), dmp, 0644); err != nil {
		return nil, err
	}
	meta := fmt.Sprintf("exec_name=%s\nver=%s\npayload=%s\ndone=1\n", exec, ver, basename+".dmp")
	if err := ioutil.WriteFile(filepath.Join(dir, basename+".meta"), []byte(meta), 0644); err != nil {
		return nil, err
	}

	lsb, err := lsbrelease.Load()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read /etc/lsb-release")
	}
	board := lsb[lsbrelease.Board]

	// On some devices like betty crossystem will fail. Fall back to "undefined" in such cases.
	out, _ := testexec.CommandContext(ctx, "crossystem", "hwid").Output()
	hwid := string(out)
	if hwid == "" {
		hwid = "undefined"
	}

	exp := &SendData{
		MetadataPath: filepath.Join(dir, basename+".meta"),
		PayloadKind:  "minidump",
		PayloadPath:  filepath.Join(dir, basename+".dmp"),
		Product:      "ChromeOS",
		Version:      ver,
		Board:        board,
		HWClass:      hwid,
		Executable:   exec,
	}
	return exp, nil
}
