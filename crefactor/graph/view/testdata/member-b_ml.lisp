;; 8 uses of buf.b_ml in 2 top-level forms: 3 read, 1 write, 2 update, 1 addr, 1 unevaluated
(member buf.b_ml
  (in get
    (:write (= (-> b b_ml) n))
    (:update (post++ (-> b b_ml)))
    (:update (+= (-> b b_ml) 2))
    (:read (return (+ (-> b b_ml) opt A))))
  (in main
    (:addr (def p (ptr int) (addr (. b b_ml))))
    (:read+unevaluated
      (return
        (+
          (. b b_ml)
          (cast int (sizeof (. b b_ml)))
          (call odd (deref p))
          (. o b_ml)
          (-> (call mk) b_ml)
          (call (index table 0) 2))))))
