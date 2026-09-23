137 declares nothing: the changedtick was a number inside a dictionary item
inside buf_T; it is the number.  Every insert, undo and search the recording
drives reads it.
