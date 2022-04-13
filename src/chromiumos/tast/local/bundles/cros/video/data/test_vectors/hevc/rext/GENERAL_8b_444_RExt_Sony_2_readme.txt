Category: general coding tools; Sub-category: "Main 4:4:4 Intra" (with general_lower_bit_rate_constraint_flag=1) profile with 8bit 4:4:4 video data

The purpose of the stream is to exercise some combinations of RExt tools in the specified profile / format
The stream consists of a concatenation of sequences, each just two pictures:
  the first of each pair has cross_component_prediction_prediction_enabled_flag=0,
  the second has cross_component_prediction_prediction_enabled_flag=1.
The concatenated sequences consist of:
  Sequence 0: transform_skip_rotation_enabled_flag=0, transform_skip_context_enabled_flag=0, implicit_rdpcm_enabled_flag=0, intra_smoothing_disabled_flag=0,
              persistent_rice_adaptation_enabled_flag=0, chroma_qp_offset_list_enabled_flag=0
  Sequence 1: as in sequence 0, but with transform_skip_rotation=1
  Sequence 2: as in sequence 0, but with transform_skip_context=1
  Sequence 3: as in sequence 0, but with implicit_rdpcm=1
  Sequence 4: as in sequence 0, but with intra_smoothing_disabled=1
  Sequence 5: as in sequence 0, but with persistent_rice_adaptation_enabled_flag=1
  Sequence 6: as in sequence 0, but with chroma_qp_offset_list_enabled_flag=1

This revision now uses a CRA slice for the second slice of each pair rather than a TRAIL_R

Level=3, tier=main

Checksum SEI messages are included.
Spec. version : HM16.3
