Bitstream file name: ipcm_D_NEC_3.hevc

Conformance point: HM-12.0

Explanation of bitstream features: Contain single coded picture. The coded picture contains only one intra slice. Both pcm_sample_bit_depth_luma_minus1 and pcm_sample_bit_depth_chroma_minus1 are equal to 7. log2_min_pcm_luma_coding_block_size_minus3, log2_diff_max_min_pcm_luma_coding_block_size, and pcm_loop_filter_disable_flag are equal to 0, 1 and 0, respectively. transquant_bypass_enable_flag is equal to 1.

Functional stage:  Test parsing of pcm_flags in coding unit syntax. Test parsing of pcm_sample_luma and pcm_sample_chroma data in PCM sample syntax. Test skipping of loop filtering on samples associated with both cu_transquant_bypass_flag and pcm_flag equal to 1.

Purpose: Check that decoder can correctly decode the slice of the coded frame containing pcm_flags, and pcm_sample_luma and pcm_sample_chroma data. Check that decoder can skip loop filtering on samples associated with both cu_transquant_bypass_flag and pcm_flag equal to 1.

Picture size: 416x240 (Minimum level of this bitstream is 2.0.)

Frame rate: 30fps
