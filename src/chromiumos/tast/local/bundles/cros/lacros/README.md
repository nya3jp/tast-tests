# How to update the lacros binary manually

There are two steps to updating the lacros binary manually. The first is to
get (build or download) a tarball with the necessary files. The second is
to upload it to GCS and update `lacros_binary.tar.external` to point to the
new tarball.

It is recommended to use a fishfood build because they have been thoroughly
tested.

## Creating the lacros tarball
### Using an existing fishfood build (recommended)

Run the following commands to grab the latest fishfood build and convert it
into a tarball usable by the Tast tests.

```sh
export BUILD=`gsutil.py ls gs://chrome-lacros-fishfood/x86_64 | sort | tail -n 1`
gsutil.py cp ${BUILD%%/}/lacros.zip lacros_binary.zip
unzip lacros_binary.zip -d lacros_binary
tar -cf lacros_binary.tar lacros_binary
rm -rI lacros_binary/ lacros_binary.zip
```

### Building manually (not recommended)

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

Make sure to delete the local binary after you are done with it, otherwise the
remote binary won't be used later when you try to test that everything worked:

```sh
rm $CROS_DIR/src/platform/tast-tests/src/chromiumos/tast/local/bundles/cros/lacros/data/lacros_binary.tar
```

## Testing and updating the lacros tarball
1. Update the lacros_binary.tar.external file:

```sh
export DATECODE=`date +"%Y%m%d"`
sha256sum lacros_binary.tar
wc -c lacros_binary.tar
echo lacros_binary_$DATECODE.tar
```

Copy-paste the SHA-256 sum and size in bytes of the file into
`$CROS_DIR/src/platform/tast-tests/src/chromiumos/tast/local/bundles/cros/lacros/data/lacros_binary.tar.external`.
Update the URL as well to the one with the correct filename.

2. Upload the new file to GCS.

```sh
export DATECODE=`date +"%Y%m%d"`
gsutil.py cp lacros_binary.tar gs://chromiumos-test-assets-public/tast/cros/lacros/lacros_binary_$DATECODE.tar
```

3. Test that Tast tests can pass with the new tarball:

```sh
tast run <dut> lacros.Basic
```

4. Upload a CL updating the `lacros_binary.tar.external` file.
