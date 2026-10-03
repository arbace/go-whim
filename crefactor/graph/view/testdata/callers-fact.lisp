;; 2 calls of fact in 2 functions
;; and 1 use not in a call, which `uses fact` shows
(callers fact
  (in @fact
    (return (* n (call fact (- n 1)))))
  (in main
    (call get (addr b) (call fact 3))))
