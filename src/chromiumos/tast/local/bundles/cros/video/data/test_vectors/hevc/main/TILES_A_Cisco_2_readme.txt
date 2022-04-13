TILES_A_Cisco_2

Specification version: HM10.0

Category: Tiles

Specification:
- Minimum level = 4.1
- 1080p60
- 100 frames
- All slices are I or P slices. Each picture contains only one slice.
- num_tiles_columns_minus1 = 4 (maximum value for level 4.1)
- num_tiles_rows_minus1 = 4 (maximum value for level 4.1)
- uniform_spacing_flag = 0
- column_width_minus1[i] = random value for each frame
- row_height_minus1[i] = random value for each frame
- loop_filter_across_tiles_enabled_flag = random value for each frame

Functional stage: Test dependency breaks at tile boundaries

Purpose: Test random non-uniform tile spacing with maximum number of tiles and disabling/enabling of deblocking filter across tile boundaries.




