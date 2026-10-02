;; The host: what the editor core asks of the world it runs in, behind a
;; record of procedures.  In whim-vim.c the host is everything from the
;; first #include to the end of the file, and the core calls it by name --
;; host_write, musl_read_input and the rest.  Here the core (the library
;; (whimsical editor), generated) still calls those names, and each is a
;; procedure of glue, the C's signature on the core's side, to the host the
;; editor runs on: editor/host.go's Host, caprice's Caprice.Host, whimsy's
;; host::Host, as a record.  What is the editor's and not the host's -- the
;; arena, which the C host keeps as the core's memory -- is the editor's
;; (whimsical rt).  So a process holds any number of editors, each on its
;; own host and thread: the terminal (whimsical term) or anything else.
(library (whimsical host)
  (export make-host host? run make-glue
          host-init host-win-size host-term-start host-term-stop host-tty-keys host-now-ms host-time
          host-delay host-wait-for-input host-read-input host-raise host-suspend host-message host-write)
  (import (chezscheme) (whimsical rt) (whimsical editor) (whimsical printf))

  ;; What the core needs of the world it runs in: a terminal, a clock,
  ;; input with a timeout, the signals, output.  The buffers are a
  ;; bytevector, a start and a count, as R6RS's ports take them.
  (define-record-type host
    (fields
      ;; (init deathtrap): start catching the signals the editor handles;
      ;; the host calls (deathtrap sig) for a SIGHUP or a SIGTERM, on the
      ;; editor's thread, where the C's handler would have run
      init
      ;; (win-size): the terminal's rows and columns, or #f #f
      win-size
      ;; (term-start), (term-stop): raw mode, and out of it
      term-start term-stop
      ;; (tty-keys fd): fd's erase and interrupt characters, and whether it
      ;; maps CR to NL on input and NL to CR NL on output; #f when fd is no
      ;; terminal
      tty-keys
      ;; (now-ms): milliseconds of a clock that starts at the first call;
      ;; (time): the Unix time
      now-ms time
      ;; (delay ms interruptible?): sleep; interruptible lets the terminal
      ;; relax meanwhile
      delay
      ;; (wait-for-input ms): whether input -- or a signal the core reads as
      ;; input -- is there within ms milliseconds (for ever when negative)
      wait-for-input
      ;; (read-input bv start count): input read into bv, the count, 0 at
      ;; its end, -1 when a signal came first; a signal the core reads as
      ;; input is written as the keys that stand for it
      read-input
      ;; (raise sig), (suspend): the process signalled, and stopped
      raise suspend
      ;; (message bv start count err?): a message outside the screen, to the
      ;; error stream when err?
      message
      ;; (write bv start count): the screen's output, the count written or -1
      write))

  ;; The C host's 17 functions, as the core calls them: each a procedure of
  ;; the editor and the C's arguments, glue to the host h -- in the order of
  ;; the core's host-names, the vector new-editor takes.
  (define (make-glue h)
    (define (int-at! ed p v) (let ([mem (ed-mem ed)]) (st-s32! p v)))
    (define procs
      `((musl_host_init . ,(lambda (ed) ((host-init h) (lambda (sig) (deathtrap ed sig)))))
        (musl_get_winsize
         . ,(lambda (ed rows cols)
              (let-values ([(r c) ((host-win-size h))])
                (cond
                  [r (int-at! ed rows r) (int-at! ed cols c) 1]   ; OK
                  [else 0]))))                                     ; FAIL
        (musl_term_start . ,(lambda (ed) ((host-term-start h))))
        (musl_term_stop . ,(lambda (ed) ((host-term-stop h))))
        (musl_tty_keys
         . ,(lambda (ed fd bs intr cr nlcr)
              (let-values ([(e i icrnl onlcr) ((host-tty-keys h) fd)])
                (cond
                  [e (int-at! ed bs e) (int-at! ed intr i)
                     (int-at! ed cr (b->i icrnl)) (int-at! ed nlcr (b->i onlcr))
                     1]
                  [else 0]))))
        (musl_now_ms . ,(lambda (ed) ((host-now-ms h))))
        (host_time . ,(lambda (ed) ((host-time h))))
        (musl_delay . ,(lambda (ed ms interruptible) ((host-delay h) ms (not (eqv? interruptible 0)))))
        (musl_wait_for_input . ,(lambda (ed ms) (b->i ((host-wait-for-input h) ms))))
        ;; no room for a negative length, and -1 for it, as read(2) of
        ;; (size_t)len answers; the host still takes the signals it reads as
        ;; input, as the C does before its read
        (musl_read_input
         . ,(lambda (ed buf len)
              (let ([n ((host-read-input h) (ed-mem ed) buf (max len 0))])
                (if (< len 0) -1 n))))
        (host_raise . ,(lambda (ed sig) ((host-raise h) sig)))
        (musl_suspend . ,(lambda (ed) ((host-suspend h))))
        (host_exit . ,(lambda (ed r) (raise-exit r)))
        (host_message
         . ,(lambda (ed msg len err)
              (let* ([mem (ed-mem ed)]
                     [n (if (< len 0)
                            (let loop ([i 0]) (if (fx= 0 (ld-u8 (fx+ msg i))) i (loop (fx+ i 1))))
                            len)])
                ((host-message h) mem msg n (not (eqv? err 0))))))
        (host_write
         . ,(lambda (ed s len)
              (cond
                [(< len 0) -1]
                [(= len 0) 0]
                [else ((host-write h) (ed-mem ed) s len)])))
        (host_alloc . ,host-alloc)
        (vim_snprintf . ,vim_snprintf)))
    (vector-map
      (lambda (name)
        (cond
          [(assq name procs) => cdr]
          [else (lambda args (error 'whimsical "the core calls a host function the host has not" name))]))
      host-names))

  ;; The C host's arena: 1 GiB of the editor's memory, never freed; its
  ;; exhaustion ends the editor, as the C's ends the process.
  (define arena-bytes (* 1024 1024 1024))
  (define (host-alloc ed n)
    (let-values ([(p used) (arena-alloc ed n)])
      (or p
          (let ([m (string->utf8
                     (format "whim-vim: host arena exhausted: ~a bytes, ~a used, request ~a\n" arena-bytes used n))]
                [glue (ed-glue ed)])
            ;; the message from a frame of the editor's stack, as the C's
            ;; from its own
            (let* ([fr (frame-push! ed (fxand (fx+ (bytevector-length m) 15) (fxnot 15)))]
                   [mem (ed-mem ed)])
              (bytevector-copy! m 0 mem fr (bytevector-length m))
              ((vector-ref glue (index-of 'host_message)) ed fr (bytevector-length m) 1)
              (raise-exit 1))))))

  (define (index-of name)
    (let loop ([i 0])
      (if (eq? (vector-ref host-names i) name) i (loop (+ i 1)))))

  ;; An editor on h, run with args -- the C's argv, the program's name first:
  ;; strings, or bytevectors of their bytes -- to its end: the status
  ;; host_exit gave, or vim_main's.  Each run is an editor of its own, its
  ;; memory and its arena, so a process may run several at once, each on its
  ;; thread.
  (define (run h args)
    (let* ([ed (new-editor (make-glue h))]
           [bytes (map (lambda (a) (if (string? a) (string->utf8 a) a)) args)]
           [argc (length bytes)]
           [size (fold-left (lambda (n b) (+ n (bytevector-length b) 1)) (* 8 (+ argc 1)) bytes)]
           ;; argv and its strings at the bottom of the editor's stack, where
           ;; the C's are above main's frame
           [argv (frame-push! ed (fxand (fx+ size 15) (fxnot 15)))]
           [mem (ed-mem ed)])
      (let loop ([i 0] [bs bytes] [at (fx+ argv (* 8 (+ argc 1)))])
        (unless (null? bs)
          (let ([b (car bs)])
            (st-ptr! (fx+ argv (fx* 8 i)) at)
            (bytevector-copy! b 0 mem at (bytevector-length b))
            (loop (fx+ i 1) (cdr bs) (fx+ at (fx+ (bytevector-length b) 1))))))
      (guard (c [(exit-condition? c) (exit-condition-code c)])
        (vim_main ed argc argv))))
)
