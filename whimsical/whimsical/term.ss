;; The terminal host: a (whimsical host) host on a terminal's file
;; descriptors, from the C host half of whim-vim.c function by function --
;; raw mode, the window's size and the keys (termios, an ioctl), the wait
;; for input, the signals, output, the clocks -- through Chez's foreign
;; procedures into musl's libc, with no C of our own (doc/SCHEME.md, §8.6).
;;
;; The signals the editor handles are blocked in every thread
;; (pthread_sigmask, before any thread is made, which inherit the mask) and
;; read from a signalfd that the wait for input polls beside the keys: a
;; handler of Chez's runs only on the main thread, at a call boundary, and a
;; poll on any other thread is not woken by one.  A signal is read where the
;; C's handler would have set its flag -- before each wait and each read --
;; so what the core sees is the C's.  The signalfd is the process's, as the
;; signals are; a process may hold several terminals, so the one that reads
;; a signal tells every terminal alive, waking each through a pipe of its
;; own, as caprice's Term does.
;;
;; A buffer handed to a foreign call is an immobile bytevector, which a
;; collection never moves: the editor's memory itself, read and written in
;; place, or one of the host's own.
(library (whimsical term)
  (export make-term-host make-term-host-on)
  (import (chezscheme) (whimsical host))

  (define libc
    (let loop ([names '("libc.musl-x86_64.so.1" "libc.so" "libc.so.6")])
      (cond
        [(null? names) (error 'whimsical "no C library to load")]
        [(guard (c [#t #f]) (load-shared-object (car names)) #t) (car names)]
        [else (loop (cdr names))])))

  (define c-read (foreign-procedure __collect_safe "read" (int uptr size_t) ssize_t))
  (define c-write (foreign-procedure __collect_safe "write" (int uptr size_t) ssize_t))
  (define c-poll (foreign-procedure __collect_safe "poll" (uptr unsigned-long int) int))
  (define c-tcgetattr (foreign-procedure "tcgetattr" (int uptr) int))
  (define c-tcsetattr (foreign-procedure "tcsetattr" (int int uptr) int))
  (define c-ioctl (foreign-procedure (__varargs_after 2) "ioctl" (int unsigned-long uptr) int))
  (define c-pipe2 (foreign-procedure "pipe2" (uptr int) int))
  (define c-signalfd (foreign-procedure "signalfd" (int uptr int) int))
  (define c-sigmask (foreign-procedure "pthread_sigmask" (int uptr uptr) int))
  (define c-signal (foreign-procedure "signal" (int uptr) uptr))
  (define c-kill (foreign-procedure "kill" (int int) int))
  (define c-getpid (foreign-procedure "getpid" () int))
  (define c-errno-location (foreign-procedure "__errno_location" () uptr))
  (define (errno) (foreign-ref 'int (c-errno-location) 0))

  ;; musl's x86_64 constants, as gcc reads them from its headers
  (define ICRNL #x100) (define IXON #x400) (define ICANON 2) (define ECHO 8) (define ISIG 1)
  (define ECHOE #x10) (define IEXTEN #x8000) (define ONLCR 4) (define XTABS #x1800)
  (define VINTR 0) (define VERASE 2) (define VTIME 5) (define VMIN 6) (define TCSANOW 0)
  (define TIOCGWINSZ #x5413) (define POLLIN 1) (define EINTR 4)
  (define SIG_BLOCK 0) (define SIG_UNBLOCK 1) (define SIG_DFL 0) (define SIG_IGN 1)
  (define O_NONBLOCK #x800) (define O_CLOEXEC #x80000)
  (define SIGHUP 1) (define SIGINT 2) (define SIGPIPE 13) (define SIGALRM 14) (define SIGTERM 15)
  (define SIGCONT 18) (define SIGTSTP 20) (define SIGWINCH 28)
  ;; struct termios: c_iflag, c_oflag, c_cflag, c_lflag, c_line, c_cc[32] at
  ;; 17; 60 bytes
  (define termios-size 60) (define iflag 0) (define oflag 4) (define lflag 12) (define cc 17)

  ;; An immobile bytevector and its address, for a foreign call.
  (define (buffer n) (make-immobile-bytevector n 0))
  (define (addr bv) (object->reference-address bv))

  ;; One terminal's state.
  (define-record-type term
    (fields in out err
            (mutable winch) (mutable tstp) (mutable int) (mutable death)
            (mutable pipe-r) (mutable pipe-w)
            saved (mutable saved?) (mutable raw?) (mutable now-base) (mutable deathtrap)
            ws pfd scratch))

  ;; The terminals alive in the process, and the process's signalfd.
  (define terms '())
  (define lock (make-mutex))
  (define sigfd -1)
  (define handled (list SIGHUP SIGTERM SIGWINCH SIGCONT SIGTSTP SIGINT))

  (define (sigset . sigs)
    (let ([s (buffer 128)])
      (for-each (lambda (sig)
                  (let ([b (fxquotient (fx- sig 1) 8)])
                    (bytevector-u8-set! s b (fxior (bytevector-u8-ref s b) (fxsll 1 (fxremainder (fx- sig 1) 8))))))
                sigs)
      s))

  ;; The terminal host on stdin, stdout and stderr: the C host's.
  (define (make-term-host) (make-term-host-on 0 1 2))

  ;; The terminal host on the given input, output and error descriptors.
  (define (make-term-host-on in out err)
    (let ([t (make-term in out err #f #f #f 0 -1 -1 (buffer termios-size) #f #f #f (lambda (sig) (void))
                        (buffer 8) (buffer 24) (buffer 128))])
      (make-host
        (lambda (deathtrap) (init t deathtrap))
        (lambda () (win-size t))
        (lambda () (term-raw?-set! t #t) (tty-set t #t #f))
        (lambda () (term-raw?-set! t #f) (tty-set t #f #f))
        tty-keys
        (lambda () (now-ms t))
        unix-time
        (lambda (ms interruptible) (pause t ms interruptible))
        (lambda (ms) (wait-for-input t ms))
        (lambda (bv start count) (read-input t bv start count))
        (lambda (sig) (c-kill (c-getpid) sig))
        suspend
        (lambda (bv start count err?) (message t bv start count err?))
        (lambda (bv start count) (write-all-once (term-out t) bv start count)))))

  ;; musl_host_init: the signals blocked and read from the signalfd, the
  ;; ones the C ignores ignored; the terminal's wake-up pipe.
  (define (init t deathtrap)
    (term-deathtrap-set! t deathtrap)
    (let ([fds (buffer 8)])
      (when (= 0 (c-pipe2 (addr fds) (fxior O_NONBLOCK O_CLOEXEC)))
        (term-pipe-r-set! t (bytevector-s32-native-ref fds 0))
        (term-pipe-w-set! t (bytevector-s32-native-ref fds 4))))
    (with-mutex lock
      (set! terms (cons t terms))
      (when (< sigfd 0)
        (let ([set (apply sigset handled)])
          (c-sigmask SIG_BLOCK (addr set) 0)
          (set! sigfd (c-signalfd -1 (addr set) (fxior O_NONBLOCK O_CLOEXEC))))
        (c-signal SIGPIPE SIG_IGN)
        (c-signal SIGALRM SIG_IGN))))

  ;; The signals that came, read from the signalfd: each terminal alive
  ;; told, as the C's handlers set their flags.
  (define siginfo (buffer 128))
  (define (take-signals)
    (when (>= sigfd 0)
      (with-mutex lock
        (let loop ()
          (when (= 128 (c-read sigfd (addr siginfo) 128))
            (let ([sig (bytevector-u32-native-ref siginfo 0)])
              (for-each
                (lambda (x)
                  (cond
                    [(or (= sig SIGHUP) (= sig SIGTERM)) (term-death-set! x sig)]
                    [(or (= sig SIGWINCH) (= sig SIGCONT)) (term-winch-set! x #t)]
                    [(= sig SIGTSTP) (term-tstp-set! x #t)]
                    [(= sig SIGINT) (term-int-set! x #t)])
                  (wake x))
                terms))
            (loop))))))

  ;; A byte down a terminal's pipe, which its wait for input watches.
  (define one (buffer 1))
  (define (wake t)
    (when (>= (term-pipe-w t) 0)
      (c-write (term-pipe-w t) (addr one) 1)))

  ;; host_deliver_death: a pending SIGHUP or SIGTERM, to the core's deathtrap.
  (define (deliver-death t)
    (take-signals)
    (when (>= (term-pipe-r t) 0)
      (let loop ()
        (when (> (c-read (term-pipe-r t) (addr (term-scratch t)) 16) 0)
          (loop))))
    (let ([sig (term-death t)])
      (unless (= sig 0)
        (term-death-set! t 0)
        ((term-deathtrap t) sig))))

  ;; host_tty_set
  (define (tty-set t raw sleep)
    (let ([saved (term-saved t)])
      (when (or (term-saved? t)
                (and (= 0 (c-tcgetattr (term-in t) (addr saved))) (begin (term-saved?-set! t #t) #t)))
        (let ([new (buffer termios-size)])
          (define (clear! off bits)
            (bytevector-u32-native-set! new off (fxand (bytevector-u32-native-ref new off) (fxnot bits))))
          (bytevector-copy! saved 0 new 0 termios-size)
          (cond
            [raw
             (clear! iflag (fxior ICRNL IXON))
             (clear! lflag (fxior ICANON ECHO ISIG ECHOE IEXTEN))
             (clear! oflag (fxior ONLCR XTABS))
             (bytevector-u8-set! new (+ cc VMIN) 1)
             (bytevector-u8-set! new (+ cc VTIME) 0)]
            [sleep
             (clear! lflag (fxior ICANON ECHO))
             (bytevector-u8-set! new (+ cc VMIN) 1)
             (bytevector-u8-set! new (+ cc VTIME) 0)])
          (let loop ([n 10])
            (when (and (= -1 (c-tcsetattr (term-in t) TCSANOW (addr new))) (= (errno) EINTR) (> n 0))
              (loop (- n 1))))))))

  ;; musl_get_winsize: the window's rows and columns, asked of the output.
  (define (win-size t)
    (let ([ws (term-ws t)])
      (if (= 0 (c-ioctl (term-out t) TIOCGWINSZ (addr ws)))
          (let ([row (bytevector-u16-native-ref ws 0)] [col (bytevector-u16-native-ref ws 2)])
            (if (or (= row 0) (= col 0)) (values #f #f) (values row col)))
          (values #f #f))))

  ;; musl_tty_keys
  (define (tty-keys fd)
    (let ([k (buffer termios-size)])
      (if (= -1 (c-tcgetattr fd (addr k)))
          (values #f #f #f #f)
          (values (bytevector-u8-ref k (+ cc VERASE))
                  (bytevector-u8-ref k (+ cc VINTR))
                  (not (= 0 (fxand (bytevector-u32-native-ref k iflag) ICRNL)))
                  (not (= 0 (fxand (bytevector-u32-native-ref k oflag) ONLCR)))))))

  ;; musl_now_ms: milliseconds since the second of the first call, as the
  ;; C's gettimeofday arithmetic has them.
  (define (now-ms t)
    (let* ([now (current-time 'time-utc)] [sec (time-second now)])
      (unless (term-now-base t) (term-now-base-set! t sec))
      (+ (* (- sec (term-now-base t)) 1000) (quotient (time-nanosecond now) 1000000))))

  ;; host_time: WHIM_TIME, when the environment holds it (phase 180), as atol
  ;; reads it; the clock's otherwise.
  (define (unix-time)
    (let ([pinned (getenv "WHIM_TIME")])
      (if (and pinned (> (string-length pinned) 0))
          (atol pinned)
          (time-second (current-time 'time-utc)))))

  (define (atol s)
    (let* ([n (string-length s)]
           [i (let skip ([i 0]) (if (and (< i n) (memv (string-ref s i) '(#\space #\tab #\newline #\vtab #\page #\return))) (skip (+ i 1)) i))]
           [neg (and (< i n) (char=? (string-ref s i) #\-))]
           [i (if (and (< i n) (memv (string-ref s i) '(#\- #\+))) (+ i 1) i)])
      (let loop ([i i] [v 0])
        (if (and (< i n) (char<=? #\0 (string-ref s i) #\9))
            (loop (+ i 1) (+ (* v 10) (- (char->integer (string-ref s i)) 48)))
            (if neg (- v) v)))))

  ;; musl_delay
  (define (pause t ms interruptible)
    (let ([relax (and interruptible (term-raw? t) (> ms 500))])
      (when relax (tty-set t #f #t))
      (when (> ms 0)
        (sleep (make-time 'time-duration (* (remainder ms 1000) 1000000) (quotient ms 1000))))
      (when relax (tty-set t #t #f))))

  ;; musl_wait_for_input: whether input is waiting within ms (for ever when
  ;; negative); true also for a signal the core must hear of.  poll(2) on
  ;; the input, the terminal's pipe and the signalfd, as the C's select on
  ;; the input and its pipe.
  (define (wait-for-input t ms)
    (let ([pfd (term-pfd t)])
      (define (slot! i fd)
        (bytevector-s32-native-set! pfd (* 8 i) fd)
        (bytevector-s16-native-set! pfd (+ (* 8 i) 4) POLLIN)
        (bytevector-s16-native-set! pfd (+ (* 8 i) 6) 0))
      (define (ready? i) (not (= 0 (bytevector-s16-native-ref pfd (+ (* 8 i) 6)))))
      (let loop ()
        (deliver-death t)
        (if (or (term-winch t) (term-tstp t) (term-int t))
            #t
            (begin
              (slot! 0 (term-in t))
              (slot! 1 (term-pipe-r t))
              (slot! 2 sigfd)
              (let ([ret (c-poll (addr pfd) 3 (if (>= ms 0) ms -1))])
                (cond
                  [(< ret 0) (if (= (errno) EINTR) (loop) #f)]
                  [(and (> ret 0) (or (ready? 1) (ready? 2))) (loop)]
                  [else (and (> ret 0) (ready? 0))])))))))

  ;; musl_read_input
  (define (read-input t bv start count)
    (deliver-death t)
    (cond
      [(and (term-int t) (>= count 1))
       (term-int-set! t #f)
       (bytevector-u8-set! bv start 3)
       1]
      [else
       (when (term-int t) (term-int-set! t #f))
       (or (and (term-winch t)
                (begin
                  (term-winch-set! t #f)
                  (let-values ([(rows cols) (win-size t)])
                    (and rows (>= count 32)
                         ;; what the C formats with vim_snprintf
                         (let ([b (string->utf8 (format "\x1b;[48;~a;~a;0;0t" rows cols))])
                           (bytevector-copy! b 0 bv start (bytevector-length b))
                           (bytevector-u8-set! bv (+ start (bytevector-length b)) 0)
                           (bytevector-length b))))))
           (and (term-tstp t)
                (begin
                  (term-tstp-set! t #f)
                  (and (>= count 5)
                       (begin
                         (bytevector-copy! #vu8(27 91 63 49 122) 0 bv start 5)
                         5))))
           (c-read (term-in t) (+ (addr bv) start) count))]))

  ;; musl_suspend: SIGTSTP let through to its default, sent, and taken back.
  (define (suspend)
    (let ([set (sigset SIGTSTP)])
      (c-sigmask SIG_UNBLOCK (addr set) 0)
      (c-kill 0 SIGTSTP)
      (c-sigmask SIG_BLOCK (addr set) 0)))

  ;; host_message: count bytes of bv, to the error stream when err?.
  (define (message t bv start count err?)
    (let ([fd (if err? (term-err t) (term-out t))])
      (let loop ([off 0])
        (when (< off count)
          (let ([w (c-write fd (+ (addr bv) start off) (- count off))])
            (when (> w 0) (loop (+ off w))))))))

  ;; host_write: one write(2), its count.
  (define (write-all-once fd bv start count)
    (c-write fd (+ (addr bv) start) count))
)
