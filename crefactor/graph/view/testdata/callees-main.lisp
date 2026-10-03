;; main calls 4 functions
(callees main
  (calls fact
    (call get (addr b) (call fact 3))
    (calls @fact
      (return (* n (call fact (- n 1))))))
  (calls odd
    (return
      (+
        (. b b_ml)
        (cast int (sizeof (. b b_ml)))
        (call odd (deref p))
        (. o b_ml)
        (-> (call mk) b_ml)
        (call (index table 0) 2)))
    (calls even
      (return (? (== n 0) 0 (call even (- n 1))))))
  (calls get
    (call get (addr b) (call fact 3)))
  (calls mk
    (return
      (+
        (. b b_ml)
        (cast int (sizeof (. b b_ml)))
        (call odd (deref p))
        (. o b_ml)
        (-> (call mk) b_ml)
        (call (index table 0) 2)))))
