;; 3 uses of opt in 2 top-level forms: 1 read, 2 write
(uses opt
  (in get
    (:write (= opt 0))
    (:read (return (+ (-> b b_ml) opt A))))
  (in main
    (:write (= opt 1))))
