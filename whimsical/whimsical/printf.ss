;; vim's own printf, vim_snprintf, from the host half of whim-vim.c: the
;; core calls it for every message it formats, and it needs nothing of an
;; operating system, so it is the library's and not the host's.  A hand port
;; of the C -- musl_memchr, musl_fmtbase, musl_fmtnum, musl_fmtptr,
;; vim_snprintf/vim_vsnprintf/vim_vsnprintf_typval, format_typeof,
;; format_typename, adjust_types, get_unsigned_int, parse_fmt_types and
;; skip_to_arg -- on the C's memory, the editor's one bytevector: the format,
;; the strings and the result are bytes at offsets into it, as in
;; editor/format.go, caprice's Caprice.Printf and whimsy's printf.rs, whose
;; structure it follows.
;;
;;     (vim_snprintf ed buf len fmt args)  ; => the int the C returns
;;
;; The C's va_list is args, the call's arguments as a list of exact integers
;; (each as the C passed it after the default promotions), and the rest of
;; it still to read: va_arg takes the head, va_copy(ap, ap_start) starts
;; again from args.  An argument is read at the type the C's va_arg names
;; (->i32 for int, ->u32 for unsigned int, ->i64 for long and varnumber_T,
;; ->u64 for the unsigned longs and void *; a char * is its offset), and
;; every C type here takes one place in the list, so skipping an argument of
;; any type is taking the head.
;;
;; What the C leaves out of reach is left out here too: get_unsigned_int is
;; only ever called with overflow_err FALSE, so it clamps and never reports,
;; and format_overflow_error, which only that report calls, is not ported;
;; nor are the returns after a get_unsigned_int that cannot fail.  The format
;; errors (E1500-E1507) are written into IObuff by this function and given to
;; the core's emsg/iemsg, as the C does.
;;
;; The C strings those errors need in the editor's memory -- the E15xx
;; formats, which the recursive vim_snprintf reads and emsg may be handed,
;; the type names they print, and "[NULL]", which %S measures with the
;; core's utf functions -- are written into the editor's arena the first
;; time a call needs one, all of them at once, and kept per memory (the
;; threads of the parallel :%s share it) in a weak table.  Every other
;; string of the C's own -- tmp, uchar_arg, the empty format -- is a
;; bytevector of this library's, or no string at all.
;;
;; DEVIATION: va_arg past the last argument -- undefined in C, a read of
;; whatever the stack holds -- reads 0 here (a null pointer for %s, which
;; prints [NULL]), as in whimsy's port, so that no format can fail here.
;;
;; DEVIATION (invisible): the positional arguments' types, ap_types, are a
;; Scheme vector of offsets into the format (#f for the C's nullptr), where
;; the C allocates them on its arena with alloc_clear/host_alloc; so the C's
;; out-of-memory return from adjust_types cannot happen, and an index past
;; num_posarg, which the C would read out of bounds, reads as nullptr.
(library (whimsical printf)
  (export vim_snprintf)
  (import (rnrs)
          (whimsical rt)
          (whimsical editor)
          (only (chezscheme) fx= fx< fx> fx<= fx>= call/1cc make-weak-eq-hashtable
                make-mutex with-mutex))

  (define TMP_LEN 350)
  (define MAX_ALLOWED_STRING_WIDTH 1048576)

  ;; --- bytes ----------------------------------------------------------------

  ;; The byte at p of the bytevector mem in scope, as a character: the C's
  ;; `*p` where the C compares it with character constants.
  (define-syntax ch-at
    (lambda (x)
      (syntax-case x ()
        [(k p)
         (with-syntax ([mem (datum->syntax #'k 'mem)])
           #'(integer->char (bytevector-u8-ref mem p)))])))

  ;; (unsigned)c - '0' < 10
  (define (digit? c) (char<=? #\0 c #\9))
  (define (digit-value c) (fx- (char->integer c) 48))

  (define (c-strlen mem p)
    (let loop ([q p])
      (if (fx= 0 (ld-u8 q)) (fx- q p) (loop (fx+ q 1)))))

  ;; Where the literal text that starts at p (not a '%', not NUL) ends: the
  ;; next '%' after p, or the NUL.  The C's
  ;;   q = strchr(p + 1, '%'); n = q == NULL ? strlen(p) : q - p;
  (define (literal-end mem p)
    (let loop ([q (fx+ p 1)])
      (case (ch-at q)
        [(#\% #\nul) q]
        [else (loop (fx+ q 1))])))

  (define (skip-digits mem p)
    (if (digit? (ch-at p)) (skip-digits mem (fx+ p 1)) p))

  ;; musl_memchr(src, c, n), as an offset or #f
  (define (musl-memchr mem src c n)
    (let loop ([s src] [n n])
      (cond
        [(fx= n 0) #f]
        [(fx= (ld-u8 s) c) s]
        [else (loop (fx+ s 1) (fx- n 1))])))

  ;; --- the conversions of numbers -----------------------------------------------

  (define (musl-fmtbase spec)
    (case spec
      [(#\o) 8]
      [(#\x #\X) 16]
      [else 10]))

  (define (hex-digit d upper)
    (cond
      [(fx< d 10) (fx+ 48 d)]
      [upper (fx+ 55 d)]       ; 'A' + d - 10
      [else (fx+ 87 d)]))      ; 'a' + d - 10

  ;; v, an unsigned long long, in base at dest of tmp, '-' first when isneg
  ;; (v then the two's complement of the magnitude); the count written, and
  ;; a NUL after it.
  (define (musl-fmtnum tmp dest v base upper isneg)
    (let digits ([v (if isneg (->u64 (- v)) v)] [ds '()])
      (let* ([d (mod v base)] [ds (cons (hex-digit d upper) ds)] [v (div v base)])
        (if (not (eqv? v 0))
            (digits v ds)
            (let ([start (if isneg (begin (bytevector-u8-set! tmp dest 45) (fx+ dest 1)) dest)])
              (let put ([ds ds] [i start])
                (if (null? ds)
                    (begin (bytevector-u8-set! tmp i 0) (fx- i dest))
                    (begin (bytevector-u8-set! tmp i (car ds)) (put (cdr ds) (fx+ i 1))))))))))

  ;; "0x" and v's sixteen hex digits at dest of tmp
  (define (musl-fmtptr tmp dest v)
    (bytevector-u8-set! tmp dest 48)
    (bytevector-u8-set! tmp (fx+ dest 1) 120)
    (do ([i 0 (fx+ i 1)])
        ((fx= i 16))
      (bytevector-u8-set! tmp (fx+ dest (fx+ 2 i))
                          (hex-digit (bitwise-and (bitwise-arithmetic-shift-right v (fx- 60 (fx* 4 i))) 15) #f)))
    (bytevector-u8-set! tmp (fx+ dest 18) 0)
    18)

  ;; bn's binary digits at dest of tmp, as the C's b[] copied there
  (define (fmt-binary tmp dest bn)
    (let digits ([bn bn] [ds '()])
      (let ([ds (cons (fx+ 48 (bitwise-and bn 1)) ds)] [bn (bitwise-arithmetic-shift-right bn 1)])
        (if (not (eqv? bn 0))
            (digits bn ds)
            (let put ([ds ds] [i dest])
              (if (null? ds)
                  (fx- i dest)
                  (begin (bytevector-u8-set! tmp i (car ds)) (put (cdr ds) (fx+ i 1)))))))))

  ;; --- the C's strings, in the editor's memory -----------------------------------

  (define c-texts
    '((unknown . "unknown")
      (int . "int")
      (long . "long int")
      (longlong . "long long int")
      (uint . "unsigned int")
      (ulong . "unsigned long int")
      (ulonglong . "unsigned long long int")
      (pointer . "pointer")
      (percent . "percent")
      (char . "char")
      (string . "string")
      (null . "[NULL]")
      (e1500 . "E1500: Cannot mix positional and non-positional arguments: %s")
      (e1501 . "E1501: format argument %d unused in $-style format: %s")
      (e1502 . "E1502: Positional argument %d used as field width reused as different type: %s/%s")
      (e1504 . "E1504: Positional argument %d type used inconsistently: %s/%s")
      (e1505 . "E1505: Invalid format specifier: %s")
      (e1507 . "E1507: Internal error: ap_types or ap_types[idx] is NULL: %d: %s")))

  ;; memory -> its strings' addresses, an eq-hashtable of c-texts' names
  (define placed (make-weak-eq-hashtable))
  (define placed-lock (make-mutex))

  ;; The address of the C string named name in ed's memory, every one of
  ;; c-texts written into the arena the first time any is asked for.
  (define (c-string ed name)
    (let ([mem (ed-mem ed)])
      (hashtable-ref
        (with-mutex placed-lock
          (or (hashtable-ref placed mem #f)
              (let*-values ([(size) (fold-left (lambda (n t) (fx+ n (fx+ (string-length (cdr t)) 1))) 0 c-texts)]
                            [(base used) (arena-alloc ed size)])
                (unless base
                  (error 'vim_snprintf "the arena has no room for printf's strings" used))
                (let ([addrs (make-eq-hashtable)])
                  (let place ([ts c-texts] [p base])
                    (unless (null? ts)
                      (let ([s (cdar ts)])
                        (mem-image! mem p s)
                        (st-u8! (fx+ p (string-length s)) 0)
                        (hashtable-set! addrs (caar ts) p)
                        (place (cdr ts) (fx+ p (fx+ (string-length s) 1))))))
                  (hashtable-set! placed mem addrs)
                  addrs))))
        name #f)))

  ;; vim_snprintf((char *)IObuff, emsg_iobuff_room(), e, args...);
  ;; emsg(iobuff_or(e)) -- or iemsg, for skip_to_arg's internal error.
  (define (format-error ed e args internal)
    (let* ([mem (ed-mem ed)]
           [ef (c-string ed e)]
           [room (emsg_iobuff_room ed)])
      (vim_snprintf ed (ld-ptr addr:IObuff) room ef args)
      (let ([s (iobuff_or ed ef)])
        (if internal (iemsg ed s) (emsg ed s)))))

  ;; --- the types of the positional arguments -------------------------------------

  ;; format_typeof: the type the conversion at t reads, as the name of its
  ;; C type in c-texts ('unknown for TYPE_UNKNOWN).
  (define (format-typeof mem t)
    (let*-values ([(lm t) (case (ch-at t)
                            [(#\h #\l)
                             (if (and (char=? (ch-at t) #\l) (char=? (ch-at (fx+ t 1)) #\l))
                                 (values #\L (fx+ t 2))
                                 (values (ch-at t) (fx+ t 1)))]
                            [else (values #\nul t)])]
                  [(spec lm) (case (ch-at t)
                               [(#\i) (values #\d lm)]
                               [(#\*) (values #\d #\h)]
                               [(#\D) (values #\d #\l)]
                               [(#\U) (values #\u #\l)]
                               [(#\O) (values #\o #\l)]
                               [else (values (ch-at t) lm)])])
      (case spec
        [(#\%) 'percent]
        [(#\c) 'char]
        [(#\s #\S) 'string]
        [(#\p) 'pointer]
        [(#\b #\B) 'ulonglong]
        [(#\d)
         (case lm [(#\nul #\h) 'int] [(#\l) 'long] [(#\L) 'longlong] [else 'unknown])]
        [(#\u #\o #\x #\X)
         (case lm [(#\nul #\h) 'uint] [(#\l) 'ulong] [(#\L) 'ulonglong] [else 'unknown])]
        [else 'unknown])))

  ;; format_typename, as the address of the name
  (define (format-typename ed t)
    (c-string ed (format-typeof (ed-mem ed) t)))

  ;; get_unsigned_int with overflow_err FALSE, its only use: the digits at p
  ;; while the value is under the limit, clamped to it, and where they end.
  ;; Its first character is taken as a digit unasked, as the C's is (the
  ;; '$' of "%$": (unsigned)('$' - '0'), past the limit).
  (define (get-unsigned-int mem p)
    (let loop ([uj (->u32 (fx- (ld-s8 p) 48))] [p (fx+ p 1)])
      (if (and (digit? (ch-at p)) (fx< uj MAX_ALLOWED_STRING_WIDTH))
          (loop (fx+ (fx* 10 uj) (digit-value (ch-at p))) (fx+ p 1))
          (values (fxmin uj MAX_ALLOWED_STRING_WIDTH) p))))

  ;; parse_fmt_types: the positional arguments' types -- a vector of offsets
  ;; into fmt, or #f when there are none (the C's nullptr) -- or 'fail after
  ;; an error has been given.
  (define (parse-fmt-types ed fmt)
    (let ([mem (ed-mem ed)] [types #f] [any-pos #f] [any-arg #f])
      (call/1cc
        (lambda (return)
          (define (fail e . args)
            (format-error ed e args #f)
            (return 'fail))
          (define (check-pos-arg)
            (when (and any-pos any-arg) (fail 'e1500 fmt)))
          ;; adjust_types
          (define (adjust arg type)
            (when (fx<= arg 0) (fail 'e1505 type))
            (when (or (not types) (fx< (vector-length types) arg))
              (let ([new (make-vector arg #f)])
                (when types
                  (do ([i 0 (fx+ i 1)]) ((fx= i (vector-length types)))
                    (vector-set! new i (vector-ref types i))))
                (set! types new)))
            (let ([prev (vector-ref types (fx- arg 1))])
              (when prev
                (if (or (char=? (ch-at prev) #\*) (char=? (ch-at type) #\*))
                    (let ([pt (if (char=? (ch-at type) #\*) prev type)])
                      (unless (memv (ch-at pt) '(#\* #\d #\i))
                        (fail 'e1502 arg (format-typename ed prev) (format-typename ed type))))
                    (unless (eq? (format-typeof mem type) (format-typeof mem prev))
                      (fail 'e1504 arg (format-typename ed type) (format-typename ed prev))))))
            (vector-set! types (fx- arg 1) type))
          ;; one conversion, p after its '%': where the next text starts
          (define (conversion p)
            (let ([pos-arg -1] [ptype (skip-digits mem p)])
              (when (char=? (ch-at ptype) #\$)
                (when (char=? (ch-at p) #\0) (fail 'e1505 fmt))
                (let-values ([(uj q) (get-unsigned-int mem p)])
                  (set! pos-arg uj)
                  (set! any-pos #t)
                  (check-pos-arg)
                  (set! p (fx+ q 1))))
              (let flags ()
                (when (memv (ch-at p) '(#\0 #\- #\+ #\space #\# #\'))
                  (set! p (fx+ p 1))
                  (flags)))
              ;; the width
              (let ([arg p])
                (cond
                  [(char=? (ch-at arg) #\*)
                   (set! p (fx+ p 1))
                   (cond
                     [(digit? (ch-at p))
                      (let-values ([(uj q) (get-unsigned-int mem p)])
                        (set! p q)
                        (unless (char=? (ch-at p) #\$) (fail 'e1505 fmt))
                        (set! p (fx+ p 1))
                        (set! any-pos #t)
                        (check-pos-arg)
                        (adjust uj arg))]
                     [else (set! any-arg #t) (check-pos-arg)])]
                  [(digit? (ch-at p))
                   (let-values ([(uj q) (get-unsigned-int mem p)])
                     (set! p q)
                     (when (char=? (ch-at p) #\$) (fail 'e1505 fmt)))]))
              ;; the precision
              (when (char=? (ch-at p) #\.)
                (set! p (fx+ p 1))
                (let ([arg p])
                  (cond
                    [(char=? (ch-at arg) #\*)
                     (set! p (fx+ p 1))
                     (cond
                       [(digit? (ch-at p))
                        (let-values ([(uj q) (get-unsigned-int mem p)])
                          (set! p q)
                          (unless (char=? (ch-at p) #\$) (fail 'e1505 fmt))
                          (set! any-pos #t)
                          (check-pos-arg)
                          (set! p (fx+ p 1))
                          (adjust uj arg))]
                       [else (set! any-arg #t) (check-pos-arg)])]
                    [(digit? (ch-at p))
                     (let-values ([(uj q) (get-unsigned-int mem p)])
                       (set! p q)
                       (when (char=? (ch-at p) #\$) (fail 'e1505 fmt)))])))
              (unless (fx= pos-arg -1)
                (set! any-pos #t)
                (check-pos-arg)
                (set! ptype p))
              (when (memv (ch-at p) '(#\h #\l))
                (let ([lm (ch-at p)])
                  (set! p (fx+ p 1))
                  (when (and (char=? lm #\l) (char=? (ch-at p) #\l))
                    (set! p (fx+ p 1)))))
              (case (ch-at p)
                [(#\i #\* #\d #\u #\o #\D #\U #\O #\x #\X #\b #\B #\c #\s #\S #\p)
                 (if (fx= pos-arg -1)
                     (begin (set! any-arg #t) (check-pos-arg))
                     (adjust pos-arg ptype))]
                [else
                 (unless (fx= pos-arg -1) (fail 'e1500 fmt))])
              (if (char=? (ch-at p) #\nul) p (fx+ p 1))))
          (unless (fx= fmt 0)
            (let loop ([p fmt])
              (case (ch-at p)
                [(#\nul) (void)]
                [(#\%) (loop (conversion (fx+ p 1)))]
                [else (loop (literal-end mem p))])))
          (when types
            (do ([i 0 (fx+ i 1)]) ((fx= i (vector-length types)))
              (unless (vector-ref types i)
                (fail 'e1501 (fx+ i 1) fmt))))
          types))))

  ;; --- vim_snprintf ---------------------------------------------------------------

  ;; vim_snprintf, vim_vsnprintf and vim_vsnprintf_typval in one: fmt
  ;; formatted with args into str, at most str_m bytes of it with the NUL;
  ;; the length the whole result would have, as an int.
  (define (vim_snprintf ed str str_m fmt args)
    (let ([types (parse-fmt-types ed fmt)])
      (if (eq? types 'fail)
          0
          (format-into ed str str_m fmt args types))))

  (define (format-into ed str str_m fmt args types)
    (let ([mem (ed-mem ed)]
          [tmp (make-bytevector TMP_LEN 0)]
          [str-l 0]
          [ap args]
          [arg-cur 0]
          [arg-idx 1])
      ;; va_arg: the next argument, 0 past the last (see DEVIATION above)
      (define (va-arg)
        (if (null? ap)
            0
            (let ([a (car ap)]) (set! ap (cdr ap)) a)))
      ;; skip_to_arg: ap at argument arg-idx (from 1), arg-cur the one
      ;; before it, arg-idx then the one after.
      (define (skip-to-arg)
        (if (fx= (fx+ arg-cur 1) arg-idx)
            (begin (set! arg-cur arg-idx) (set! arg-idx (fx+ arg-idx 1)))
            (let ([arg-min (if (fx>= arg-cur arg-idx) (begin (set! ap args) 0) arg-cur)])
              (let loop ([cur arg-min])
                (set! arg-cur cur)
                (if (fx< cur (fx- arg-idx 1))
                    (let ([t (and types (fx< cur (vector-length types)) (vector-ref types cur))])
                      (if t
                          (begin
                            (unless (memq (format-typeof mem t) '(percent unknown))
                              (va-arg))
                            (loop (fx+ cur 1)))
                          (format-error ed 'e1507 (list cur fmt) #t)))
                    (begin (set! arg-cur (fx+ cur 1)) (set! arg-idx (fx+ arg-idx 1))))))))
      ;; skip_to_arg then va_arg, at the C's types
      (define (arg) (skip-to-arg) (va-arg))
      (define (int-arg) (->i32 (arg)))
      ;; n bytes of src from off, as much of them as str has room for; and n
      ;; bytes c the same way.  Both count all n.
      (define (put! src off n)
        (when (< str-l str_m)
          (bytevector-copy! src off mem (+ str str-l) (min n (- str_m str-l))))
        (set! str-l (+ str-l n)))
      (define (fill! c n)
        (when (< str-l str_m)
          (mem-fill! (+ str str-l) (char->integer c) (min n (- str_m str-l))))
        (set! str-l (+ str-l n)))
      ;; one conversion, p after its '%': where the text after it starts
      (define (conversion p)
        (let ([min-field-width 0]
              [precision 0]
              [zero-padding #f]
              [precision-specified #f]
              [justify-left #f]
              [alternate-form #f]
              [force-sign #f]
              [space-for-positive #t]
              [length-modifier #\nul]
              [str-arg mem]          ; the bytevector str_arg points into,
              [str-arg-off 0]        ; and where
              [str-arg-l 0]
              [number-of-zeros-to-pad 0]
              [zero-padding-insertion-ind 0]
              [fmt-spec #\nul]
              [pos-arg -1])
          ;; %N$: the argument's position
          (when (char=? (ch-at (skip-digits mem p)) #\$)
            (let-values ([(uj q) (get-unsigned-int mem p)])
              (set! pos-arg uj)
              (set! p (fx+ q 1))))
          (let flags ()
            (define (next) (set! p (fx+ p 1)) (flags))
            (case (ch-at p)
              [(#\0) (set! zero-padding #t) (next)]
              [(#\-) (set! justify-left #t) (next)]
              [(#\+) (set! force-sign #t) (set! space-for-positive #f) (next)]
              [(#\space) (set! force-sign #t) (next)]
              [(#\#) (set! alternate-form #t) (next)]
              [(#\') (next)]
              [else (void)]))
          ;; the width
          (cond
            [(char=? (ch-at p) #\*)
             (set! p (fx+ p 1))
             (when (digit? (ch-at p))
               (let-values ([(uj q) (get-unsigned-int mem p)])
                 (set! arg-idx uj)
                 (set! p (fx+ q 1))))
             (let ([j (fxmin (int-arg) MAX_ALLOWED_STRING_WIDTH)])
               (if (fx>= j 0)
                   (set! min-field-width j)
                   (begin
                     (set! min-field-width (->u64 (->i32 (fx- 0 j))))
                     (set! justify-left #t))))]
            [(digit? (ch-at p))
             (let-values ([(uj q) (get-unsigned-int mem p)])
               (set! min-field-width uj)
               (set! p q))])
          ;; the precision
          (when (char=? (ch-at p) #\.)
            (set! p (fx+ p 1))
            (set! precision-specified #t)
            (cond
              [(digit? (ch-at p))
               (let-values ([(uj q) (get-unsigned-int mem p)])
                 (set! precision uj)
                 (set! p q))]
              [(char=? (ch-at p) #\*)
               (set! p (fx+ p 1))
               (when (digit? (ch-at p))
                 (let-values ([(uj q) (get-unsigned-int mem p)])
                   (set! arg-idx uj)
                   (set! p (fx+ q 1))))
               (let ([j (fxmin (int-arg) MAX_ALLOWED_STRING_WIDTH)])
                 (if (fx>= j 0)
                     (set! precision j)
                     (begin (set! precision-specified #f) (set! precision 0))))]))
          ;; the length modifier
          (when (memv (ch-at p) '(#\h #\l))
            (set! length-modifier (ch-at p))
            (set! p (fx+ p 1))
            (when (and (char=? length-modifier #\l) (char=? (ch-at p) #\l))
              (set! length-modifier #\L)
              (set! p (fx+ p 1))))
          (set! fmt-spec (ch-at p))
          (case fmt-spec
            [(#\i) (set! fmt-spec #\d)]
            [(#\D) (set! fmt-spec #\d) (set! length-modifier #\l)]
            [(#\U) (set! fmt-spec #\u) (set! length-modifier #\l)]
            [(#\O) (set! fmt-spec #\o) (set! length-modifier #\l)])
          (unless (fx= pos-arg -1)
            (set! arg-idx pos-arg))
          (case fmt-spec
            [(#\% #\c #\s #\S)
             (set! str-arg-l 1)
             (case fmt-spec
               [(#\%) (set! str-arg-off p)]
               [(#\c)
                (bytevector-u8-set! tmp 0 (->u8 (int-arg)))   ; uchar_arg
                (set! str-arg tmp)]
               [else
                (let ([s (arg)])
                  (cond
                    [(eqv? s 0)
                     (set! str-arg-off (c-string ed 'null))
                     (set! str-arg-l 6)]
                    [else
                     (set! str-arg-off s)
                     (set! str-arg-l
                       (cond
                         [(not precision-specified) (c-strlen mem s)]
                         [(eqv? precision 0) 0]
                         [else
                          (let ([q (musl-memchr mem s 0 (min precision #x7fffffff))])
                            (if q (fx- q s) precision))]))]))
                (when (char=? fmt-spec #\S)
                  ;; the cells, not the bytes, are the precision and the width
                  (let loop ([p1 str-arg-off] [i 0])
                    (let ([cell (and (not (fx= 0 (ld-u8 p1))) (utf_ptr2cells ed p1))])
                      (if (and cell (not (and precision-specified (> (+ i cell) precision))))
                          (loop (fx+ p1 (utfc_ptr2len ed p1)) (+ i cell))
                          (begin
                            (set! str-arg-l (fx- p1 str-arg-off))
                            (unless (eqv? min-field-width 0)
                              (set! min-field-width (->u64 (+ min-field-width (- str-arg-l i))))))))))])]
            [(#\d #\u #\b #\B #\o #\x #\X #\p)
             ;; the argument, at its type, and its sign (0 for any unsigned 0)
             (let*-values
               ([(v arg-sign)
                 (let ([signed (lambda (v) (values v (cond [(> v 0) 1] [(< v 0) -1] [else 0])))]
                       [unsigned (lambda (v) (values v (if (eqv? v 0) 0 1)))])
                   (case fmt-spec
                     [(#\p) (set! length-modifier #\nul) (unsigned (->u64 (arg)))]
                     [(#\b #\B) (unsigned (->u64 (arg)))]
                     [(#\d)
                      (case length-modifier
                        [(#\nul #\h) (signed (->i32 (arg)))]
                        [else (signed (->i64 (arg)))])]
                     [else
                      (case length-modifier
                        [(#\nul #\h) (unsigned (->u32 (arg)))]
                        [else (unsigned (->u64 (arg)))])]))])
               (set! str-arg tmp)
               (when precision-specified
                 (set! zero-padding #f))
               (cond
                 [(char=? fmt-spec #\d)
                  (when (and force-sign (fx>= arg-sign 0))
                    (bytevector-u8-set! tmp str-arg-l (if space-for-positive 32 43))
                    (set! str-arg-l (fx+ str-arg-l 1)))]
                 [alternate-form
                  (when (and (not (fx= arg-sign 0)) (memv fmt-spec '(#\b #\B #\x #\X)))
                    (bytevector-u8-set! tmp str-arg-l 48)
                    (bytevector-u8-set! tmp (fx+ str-arg-l 1) (char->integer fmt-spec))
                    (set! str-arg-l (fx+ str-arg-l 2)))])
               (set! zero-padding-insertion-ind str-arg-l)
               (unless precision-specified
                 (set! precision 1))
               (unless (and (eqv? precision 0) (fx= arg-sign 0))
                 (set! str-arg-l
                   (fx+ str-arg-l
                        (case fmt-spec
                          [(#\p) (musl-fmtptr tmp str-arg-l v)]
                          [(#\b #\B) (fmt-binary tmp str-arg-l v)]
                          [(#\d)
                           (let ([v (if (char=? length-modifier #\h) (->i16 v) v)])
                             (musl-fmtnum tmp str-arg-l (->u64 v) 10 #f (< v 0)))]
                          [else
                           (musl-fmtnum tmp str-arg-l (if (char=? length-modifier #\h) (->u16 v) v)
                                        (musl-fmtbase fmt-spec) (char=? fmt-spec #\X) #f)])))
                 (when (and (fx< zero-padding-insertion-ind str-arg-l)
                            (fx= (bytevector-u8-ref tmp zero-padding-insertion-ind) 45))
                   (set! zero-padding-insertion-ind (fx+ zero-padding-insertion-ind 1)))
                 (when (and (fx< (fx+ zero-padding-insertion-ind 1) str-arg-l)
                            (fx= (bytevector-u8-ref tmp zero-padding-insertion-ind) 48)
                            (memv (bytevector-u8-ref tmp (fx+ zero-padding-insertion-ind 1)) '(120 88)))
                   (set! zero-padding-insertion-ind (fx+ zero-padding-insertion-ind 2))))
               (let ([num-of-digits (fx- str-arg-l zero-padding-insertion-ind)])
                 (when (and alternate-form (char=? fmt-spec #\o)
                            (not (and (fx< zero-padding-insertion-ind str-arg-l)
                                      (fx= (bytevector-u8-ref tmp zero-padding-insertion-ind) 48))))
                   (when (or (not precision-specified) (< precision (fx+ num-of-digits 1)))
                     (set! precision (fx+ num-of-digits 1))))
                 (when (< num-of-digits precision)
                   (set! number-of-zeros-to-pad (- precision num-of-digits))))
               (when (and (not justify-left) zero-padding)
                 (let ([n (->i32 (- min-field-width (+ str-arg-l number-of-zeros-to-pad)))])
                   (when (fx> n 0)
                     (set! number-of-zeros-to-pad (+ number-of-zeros-to-pad n))))))]
            [else
             (set! zero-padding #f)
             (set! justify-left #t)
             (set! min-field-width 0)
             (set! str-arg-off p)
             (set! str-arg-l (if (char=? (ch-at p) #\nul) 0 1))])
          (unless (char=? (ch-at p) #\nul)
            (set! p (fx+ p 1)))
          ;; the padding, the zeros, the text
          (let ([pad (lambda (c)
                       (let ([pn (->i32 (- min-field-width (+ str-arg-l number-of-zeros-to-pad)))])
                         (when (fx> pn 0) (fill! c pn))))])
            (unless justify-left
              (pad (if zero-padding #\0 #\space)))
            (if (eqv? number-of-zeros-to-pad 0)
                (set! zero-padding-insertion-ind 0)
                (let ([zn (->i32 zero-padding-insertion-ind)])
                  (when (fx> zn 0) (put! str-arg str-arg-off zn))
                  (let ([zn (->i32 number-of-zeros-to-pad)])
                    (when (fx> zn 0) (fill! #\0 zn)))))
            (let ([sn (->i32 (- str-arg-l zero-padding-insertion-ind))])
              (when (fx> sn 0)
                (put! str-arg (fx+ str-arg-off zero-padding-insertion-ind) sn)))
            (when justify-left
              (pad #\space)))
          p))
      (unless (fx= fmt 0)
        (let loop ([p fmt])
          (case (ch-at p)
            [(#\nul) (void)]
            [(#\%) (loop (conversion (fx+ p 1)))]
            [else
             (let ([q (literal-end mem p)])
               (put! mem p (fx- q p))
               (loop q))])))
      (when (> str_m 0)
        (st-u8! (+ str (min str-l (- str_m 1))) 0))
      (->i32 str-l)))
)
