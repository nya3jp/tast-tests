Bitstream file name: LTRPSPS_A_Qualcomm_1.hevc

Conformance point: HM-12.0

Explanation of bitstream features: The bitstream is coded under the random access conditions described in the CTC, with the following modifications. Eight long-term reference picture candidates (four difference POC LSB values and 2 values of used_by_curr, giving a total of 8) are signaled in the SPS. The slice headers refer to long-term reference pictures that are either referred to from the SPS or may be explicitly signaled in the slice header. Reference picture list modification is applied on some pictures.

Functional stage: Test parsing of long_term_ref_pics_present_flag, num_long_term_ref_pics_sps, lt_ref_pic_poc_lsb_sps, and used_by_curr_pic_lt_sps_flag in SPS. Test parsing of num_long_term_sps and lt_idx_sps in slice header syntax.

Purpose: Check whether the decoder can decode slice headers when the long-term reference pictures from the list of candidates in the SPS are referred to.

Picture size: 416x240

Frame rate: 50 fps
