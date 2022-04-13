ENTP_B_Qualcomm_1

Specification version: HM13.0

Category: Entry Point
Replacement for ENTP_X_LG_X conformance test bitstreams

Specification:
- 1080p60
- 24 frames
- Random access configuration
- One slice per picture
- Six tiles per picture
	* num_tiles_columns_minus1 = 1
	* num_tiles_rows_minus1 = 2
	* uniform_spacing_idc = 1
- MD5 checksum is included in the bitstream
NOTE: There are some pictures (e.g., POC 4, 6, 10, 12, 18, and 20) contains a tile in which emulation prevention bytes occur.

Purpose: Test entry point signalling in slice header when tile is used and when emulation prevention bytes occur in substream(s)
