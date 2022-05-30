TODO(andrewlassalle): Ideally, The pbf files should be compiled automatically,
but that is not supported when they are part of the tast packages. For that,
we would need to move all the prototxt to their own package, and the tast test
will depend on that package. The only issue there is that the prototxt files
will not be easily accessible for quick modifications.
In the meantime, we can generate the pbf files by running the following command
outside the chroot:

```
rm *.pbf && for ff in *.prototxt; do gqui from textproto:"${ff%.*}".prototxt proto ~/chromiumos/src/platform2/shill/mobile_operator_db/mobile_operator_db.proto:MobileOperatorDB --outfile=rawproto:"${ff%.*}".pbf; done
```