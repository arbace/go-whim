(follow b
  (to buf_T
    (next (ptr (struct buf))))
  (to (struct other)
    (owner (ptr buf_T)))
  (to get
    (b (ptr buf_T))
    (b (ptr buf_T))
    (-> b next))
  (to mk
    (addr one))
  (to main
    (addr b)
    (call mk)))
