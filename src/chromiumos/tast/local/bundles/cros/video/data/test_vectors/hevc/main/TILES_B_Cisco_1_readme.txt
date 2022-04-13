TILES_B_Cisco_1

Specification version: HM10.0

Category: Tiles

Specification:
- Minimum level = 4.1
- 1080p60
- 100 frames
- All slices are I or P slices.
- Each picture contains a random number of slices.
- All slice boundaries aligned with tile boundaries.
- num_tiles_columns_minus1                     = 4 (maximum value for level 4.1)
- num_tiles_rows_minus1                        = 4 (maximum value for level 4.1)
- uniform_spacing_flag                         = 0
- column_width_minus1[i]                       = random value for each frame
- row_height_minus1[i]                         = random value for each frame
- loop_filter_across_tiles_enabled_flag        = random value for each frame
- pps_loop_filter_across_slices_enabled_flag   = random value for each frame
- slice_loop_filter_across_slices_enabled_flag = random value for each slice

Functional stage: Test dependency breaks at tile boundaries and enabling/disabling the deblocking filter at tile/slice boundaries.

Purpose: Test random non-uniform tile spacing with maximum number of tiles and randomly disabling/enabling the deblocking filter across tile/slice boundaries.



