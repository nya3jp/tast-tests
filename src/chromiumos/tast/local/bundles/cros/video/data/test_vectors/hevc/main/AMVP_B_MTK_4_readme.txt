Software version: HM-10.0

Description :
All slices are coded as I or B slices. Each picture contains only one slice.
Multiple reference pictures are used. For GPB slices,  num_ref_idx_l0_default_active_minus1 is  equal to 3 and num_ref_idx_active_override_flag is equal to 1.
For non-GPB B slices,  num_ref_idx_l0_default_active_minus1 is  equal to 1, num_ref_idx_l1_default_active_minus1 is  equal to 1 and num_ref_idx_active_override_flag is equal to 0

Purpose :
Check whether motion vector prediction candidate generation and signalling perform correctly when neighboring PUs have various partition mode (part_mode), prediction mode (pred_mode_flag), reference index (ref_idx_l0, ref_idx_l1), inter_pred_idc.
