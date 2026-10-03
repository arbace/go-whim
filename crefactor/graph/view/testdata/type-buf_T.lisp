;; struct buf: 2 members, taken by 1 function, returned by 1, held in 1 member, 0 objects
(type (struct buf)
  (members
    (member b_ml int (uses 8 (read 3) (write 1) (update 2) (addr 1) (unevaluated 1)))
    (member next (ptr (struct buf)) (uses 1 (read 1))))
  (taking
    (fn get (b (ptr buf_T))))
  (returning
    (fn mk (ptr buf_T)))
  (held-in
    (member other.owner (ptr buf_T))))
