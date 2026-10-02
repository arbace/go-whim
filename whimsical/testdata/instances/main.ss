;; Editors are instances: four run at once in one process, each on a thread
;; and a host of this program's own -- keys from a bytevector, the screen
;; into a list, a fixed size -- and none sees another's text.
;; editor/host_test.go's TestEditorsAreInstances, in Scheme;
;; whimsical_test.go runs it on the libraries `make bin/whimsical` compiled.
(import (chezscheme) (whimsical host))

;; A host with no operating system under it: what an embedding program
;; would write.
(define (fake-host keys screen messages)
  (let ([at 0])
    (make-host
      (lambda (deathtrap) (void))                          ; init
      (lambda () (values 24 80))                           ; win-size
      (lambda () (void)) (lambda () (void))                ; term-start, term-stop
      (lambda (fd) (values #x7f 3 #t #t))                  ; tty-keys
      (lambda () 0) (lambda () 0)                          ; now-ms, time
      (lambda (ms interruptible?) (void))                  ; delay
      (lambda (ms) (< at (bytevector-length keys)))        ; wait-for-input
      (lambda (bv start count)                             ; read-input
        (let ([n (min count (- (bytevector-length keys) at))])
          (bytevector-copy! keys at bv start n)
          (set! at (+ at n))
          n))
      (lambda (sig) (void)) (lambda () (void))             ; raise, suspend
      (lambda (bv start count err?)                        ; message
        (set-box! messages (cons (bytes bv start count) (unbox messages))))
      (lambda (bv start count)                             ; write
        (set-box! screen (cons (bytes bv start count) (unbox screen)))
        count))))

(define (bytes bv start count)
  (let ([b (make-bytevector count)])
    (bytevector-copy! bv start b 0 count)
    b))

(define (text i) (format "editing number ~a" i))

(define (contains? hay needle)
  (let ([n (string-length needle)] [m (string-length hay)])
    (let loop ([i 0])
      (cond
        [(> (+ i n) m) #f]
        [(string=? (substring hay i (+ i n)) needle) #t]
        [else (loop (+ i 1))]))))

(define (latin1 bvs)
  (list->string (map integer->char (apply append (map bytevector->u8-list (reverse bvs))))))

(define n 4)
(define runs
  (map (lambda (i)
         (let ([screen (box '())] [messages (box '())] [status (box #f)])
           (list screen messages status
                 (fork-thread
                   (lambda ()
                     (set-box! status
                       (run (fake-host (string->utf8 (string-append "i" (text i) "\x1b;:q!\r")) screen messages)
                            '("whimsical"))))))))
       (iota n)))

(define bad #f)
(for-each
  (lambda (i r)
    (let ([screen (car r)] [messages (cadr r)] [status (caddr r)])
      (thread-join (cadddr r))
      (unless (eqv? (unbox status) 0)
        (printf "editor ~a: status ~s; messages ~s\n" i (unbox status) (latin1 (unbox messages)))
        (set! bad #t))
      (let ([s (latin1 (unbox screen))])
        (for-each
          (lambda (j)
            (unless (eq? (contains? s (text j)) (= i j))
              (printf "editor ~a's screen shows editor ~a's text: ~a\n" i j (contains? s (text j)))
              (set! bad #t)))
          (iota n)))))
  (iota n) runs)
(if bad
    (exit 1)
    (printf "ok: ~a editors, each its own\n" n))
