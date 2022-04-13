Test bitstreams #SAO_H

Specification: All slices are coded as I slices. Each picture contains multiple slices. SAO edge modes 2 and 3 are used (diagonal). slice_loop_filter_across_slices_enabled_flag is set to 0.

Functional stage: Tests SAO decoding process with different diagonal neighbour availabilty.

Purpose: Check that, when slice_loop_filter_across_slices_enabled_flag is set to zero, the decoder can properly apply the SAO filter in CTU corners whether diagonally neighbouring CTUs are available or not.
