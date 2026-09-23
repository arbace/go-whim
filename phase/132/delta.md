132 declares nothing: host_free() has had an empty body since phase 124, so no
call to it or to vim_free() did anything.  All 273 go; the check computes the
whole output from the input and compares it byte for byte.
