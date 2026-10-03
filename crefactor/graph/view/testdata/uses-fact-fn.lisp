;; 3 uses of fact in 3 top-level forms: 2 call, 1 read
(uses fact
  (in @fact
    (:call
      (defn static fact (fn ((n int)) int)
        (if (< n 2) (block (return 1)))
        (return (* n (call fact (- n 1)))))))
  (in table
    (:read (def static table (array (ptr (fn (int) int))) (init fact odd))))
  (in main
    (:call
      (defn main (fn (void) int)
        (def b buf_T)
        (def o (struct other))
        (= (. o b_ml) 1)
        (def p (ptr int) (addr (. b b_ml)))
        (= opt 1)
        (call get (addr b) (call fact 3))
        (return
          (+
            (. b b_ml)
            (cast int (sizeof (. b b_ml)))
            (call odd (deref p))
            (. o b_ml)
            (-> (call mk) b_ml)
            (call (index table 0) 2)))))))
