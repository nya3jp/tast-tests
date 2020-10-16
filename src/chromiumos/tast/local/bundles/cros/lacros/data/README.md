# How to update the lacros binary manually

1. Build lacros. Make sure dcheck_always_on=false.

```sh
ninja -C out/Lacros -j 16000 -l 400 chrome
```

2. Create a tarball with the needed lacros files.

```sh
export LACROS_OUTDIR=<your out dir> # e.g. out/Lacros
cd $LACROS_OUTDIR
mkdir lacros_binary
cp -r {chrome,nacl_helper,nacl_irt_x86_64.nexe,locales,*.pak,icudtl.dat,snapshot_blob.bin,swiftshader,crashpad_handler} lacros_binary
tar -cf lacros_binary.tar lacros_binary
```

The content of the tarball should be a single 'lacros_binary' directory with the
files inside.

3. Optionally test the lacros binary locally:

```sh
export CROS_DIR=<your chromeos dir> # e.g. /src/cros
cp lacros_binary.tar $CROS_DIR/src/platform/tast-tests/src/chromiumos/tast/local/bundles/cros/lacros/data
rm $CROS_DIR/src/platform/tast-tests/src/chromiumos/tast/local/bundles/cros/lacros/data/lacros_binary.tar.external
```
Then from inside the ChromiumOS chroot:

```sh
tast run <dut> lacros.Basic
```

4. Update the lacros_binary.tar.external file:

```sh
export DATECODE=`date +"%Y%m%d"`
sha256sum lacros_binary.tar
wc -c lacros_binary.tar
echo lacros_binary_$DATECODE.tar
```

Copy-paste the SHA-256 sum and size in bytes of the file into
`$CROS_DIR/src/platform/tast-tests/src/chromiumos/tast/local/bundles/cros/lacros/data/lacros_binary.tar.external`.
Update the URL as well to the one with the correct filename.

5. Upload the new file to GCS.

```sh
export DATECODE=`date +"%Y%m%d"`
gsutil.py cp lacros_binary.tar gs://chromiumos-test-assets-public/tast/cros/lacros/lacros_binary_$DATECODE.tar
```

6. Test that Tast tests can pass with the new tarball:

```sh
tast run <dut> lacros.Basic
```

7. Upload a CL updating the `lacros_binary.tar.external` file.
