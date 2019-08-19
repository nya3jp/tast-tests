package usb

import (
	"context"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
)

// InstallModules installs the "usb_f_fs" kernel module.
func InstallModules(ctx context.Context) error {
//	cmd := testexec.CommandContext(ctx, "modprobe", "-a", "usb_f_fs", "dummy_hcd")
	cmd := testexec.CommandContext(ctx, "modprobe", "-a", "usb_f_fs", "usb_f_mass_storage", "dummy_hcd")
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "failed to install usb kernel module")
	}
	return nil
}

// RemoveModules removes the "usb_f_fs" kernel module.
func RemoveModules(ctx context.Context) error {
	cmd := testexec.CommandContext(ctx, "modprobe", "-r", "-a", "usb_f_fs", "usb_f_mass_storage", "dummy_hcd")
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "failed to remove usb kernel modules")
	}
	return nil
}

func OpenUsb(bus, port int) (fd uintptr, err error) {
//	if f, err := os.OpenFile(name, os.O_RDWR, 0644); err != nil {
//		fd = f.Fd()
//	}
	return
}

