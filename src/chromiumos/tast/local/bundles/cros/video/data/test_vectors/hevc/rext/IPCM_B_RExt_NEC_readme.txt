Bitstream file name: IPCM_B_RExt_NEC_1.bit

Conformance point: HM-14.0

Explanation of bitstream features: Contain single coded picture. The coded picture contains only one intra slice. pcm_enabled_flag is equal to 1. Both pcm_sample_bit_depth_luma_minus1 and pcm_sample_bit_depth_chroma_minus1 are equal to 7. log2_min_pcm_luma_coding_block_size_minus3, log2_diff_max_min_pcm_luma_coding_block_size, and pcm_loop_filter_disable_flag are equal to 0, 1 and 0, respectively.

Functional stage: Test parsing of pcm_flags in coding unit syntax. Test parsing of 4:4:4 format pcm_sample_luma and pcm_sample_chroma data in PCM sample syntax.

Purpose: Check that decoder can correctly decode the slice of coded frames containing pcm_flags. 

Picture size: 416x240 (Minimum level of this bitstream is 2.0.)

Frame rate: 30fps
