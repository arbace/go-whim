;; 1 call of even in 1 function
(callers even
  (in odd
    (return (? (== n 0) 0 (call even (- n 1))))
    (in @even
      (return (? (== n 0) 1 (call odd (- n 1)))))
    (in main
      (return
        (+
          (. b b_ml)
          (cast int (sizeof (. b b_ml)))
          (call odd (deref p))
          (. o b_ml)
          (-> (call mk) b_ml)
          (call (index table 0) 2))))))
