# VM tests

This package contains general tests for VMs. Tests specific to
[Crostini](/src/chromiumos/tast/local/bundles/cros/crostini) and
[ARC](/src/chromiumos/tast/local/bundles/cros/arc) are in their respective
packages.

## Data files

The `vm` package includes VM images as external data files suitable for
exercising crosvm functionality. For now, only [Termina] images are included
for both `x86_64` and `arm64`.

> NOTE: Because the included [Termina] data files are artifacts from CrOS
> builders, they use the platform version number as a suffix rather than the
> date. If updating the data files, please continue to use the version suffix
> as it's much more meaningful than the date.

[Termina]: https://chromium.googlesource.com/chromiumos/overlays/board-overlays/+/refs/heads/main/project-termina/
