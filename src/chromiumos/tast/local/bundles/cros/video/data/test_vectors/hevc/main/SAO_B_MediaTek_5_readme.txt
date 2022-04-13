SAO_B_MediaTek_1
Spec version: HM-10.0
with fix for #1071 (https://hevc.hhi.fraunhofer.de/trac/hevc/changeset/3408)
fix PROFILE_LEVEL_INDICATOR

(Category: SAO; sub categories: Tile- and component-level control)

The purpose of the stream is to exercise tiles / randomly enabling SAO Y and/or SAO UV per slice
The bitstream contains I, B slices and one slice per picture. Checksum SEI messages are included in bitstream.