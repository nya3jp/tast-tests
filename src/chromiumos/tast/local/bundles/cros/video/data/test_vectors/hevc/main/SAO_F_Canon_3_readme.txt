Specification: All slices are coded as I or B slices. Each picture contains only one slice. A set of SAO parameters is associated to each CTB for all frames. This means that no SAO merge flag (up or left) is used. Only the Band Offset (BO) SAO type is used and the 4 BO offsets are forced to be -7 or 7 in a random way. The Coded Tree Block size is set to 32x32.

Functionnal stage: Tests loading of maximum SAO information at CTB level and frame.

Purpose: Check that decoder can properly decode slices of coded frames with full SAO information.