;; The runtime the generated editor is written against (doc/SCHEME.md): C's
;; memory, kept as C keeps it.  Every C object of an editor lives in one
;; bytevector, laid out as the C lays it out on amd64 -- below 64 KiB the
;; null page and the function pointers, then the file-scope objects and the
;; string literals, then the main thread's stack of call frames, the
;; arena the host allocates from, and the stacks of the threads the
;; parallel :%s runs on -- and a pointer is an offset into it, a fixnum.
;; So the translation reads and writes bytes at offsets, and C's pointer
;; arithmetic, comparison and punning are the machine's.
;;
;; C's integers are exact integers in their type's range, wrapped where C
;; wraps them (gcc -O0's signed overflow included): the 8-, 16- and 32-bit
;; types on fixnums, `long` and `unsigned long` with a fixnum test and a
;; bignum only past 2^60.  A C bool is #t or #f, a byte 0 or 1 in memory.
(library (whimsical rt)
  (export
    ;; the editor
    ed? ed-mem ed-sp ed-sp-set! ed-glue make-editor ed-fork
    exit-condition? exit-condition-code raise-exit
    arena-alloc
    ;; memory: the bytevector bound as mem where these are used
    ld-s8 ld-u8 ld-s16 ld-u16 ld-s32 ld-u32 ld-s64 ld-u64 ld-ptr ld-bool
    st-s8! st-u8! st-s16! st-u16! st-s32! st-u32! st-s64! st-u64! st-ptr! st-bool!
    mem-copy! mem-zero! mem-fill! mem-image! mem-bytes mem-string
    mem-ref mem-set! define-c-object define-c-local define-c-member define-c-enum c-case c-enum
    frame-push! frame-pop! c-str
    ;; C's integers
    ->i8 ->u8 ->i16 ->u16 ->i32 ->u32 ->i64 ->u64 b->i
    i32+ i32- i32* i32/ i32% i32<< i32>>
    u32+ u32- u32* u32/ u32% u32<< u32>>
    i64+ i64- i64* i64/ i64% i64<< i64>>
    u64+ u64- u64* u64/ u64% u64<< u64>>
    u32~ u64~
    ;; function pointers
    fn-ptr fn-index ch
    void
    ;; the parallel loop
    chunks
    ;; Chez's own, for the core
    fxquotient)
  (import (rnrs) (rnrs mutable-strings) (only (chezscheme) define-property fxsll/wraparound fx*/wraparound fxquotient fxremainder fx= fx< fx> fx<= fx>=
                        fxsra fxsrl fx1+
                        quotient remainder void make-immobile-bytevector
                        fork-thread thread-join make-mutex with-mutex mutex-acquire mutex-release
                        bytevector-truncate! format getenv))

  ;; --- the editor ----------------------------------------------------------

  ;; An editor: its memory, the stack its thread's frames are on, the
  ;; host's functions (a vector, in the order the core's host-names lists
  ;; them), and what its threads share.  ed-fork makes the record a thread
  ;; of the parallel :%s runs with: the same memory, a stack of its own.
  (define-record-type ed
    (fields mem (mutable sp) limit glue shared))

  ;; What an editor's threads share: the arena's bump pointer and end, how
  ;; far it is zeroed, the lock they take, and the stacks the threads of
  ;; the parallel loop run on.
  (define-record-type shared
    (fields arena-base (mutable next) end (mutable zeroed) lock (mutable stacks) stack-base stack-end
            (mutable stack-next)))

  (define stack-bytes (* 8 1024 1024))       ; the main thread's frames
  (define thread-stack-bytes (* 1024 1024))  ; each worker's
  (define thread-stacks 128)
  (define arena-bytes (* 1024 1024 1024))    ; the C host's HOST_ARENA_BYTES

  (define (align16 n) (fxand (fx+ n 15) (fxnot 15)))

  ;; An editor whose memory holds data-end bytes of the core's (the null
  ;; page, the segment and the literals, which the core then fills in),
  ;; on the host's functions glue.  The bytevector is made unfilled: its
  ;; pages are the kernel's zeros until touched, and what the editor is
  ;; handed -- the data, a frame, the arena as it grows -- is zeroed first.
  (define (make-editor glue data-end)
    (let* ([stack (align16 data-end)]
           [arena (fx+ stack stack-bytes)]
           [stacks (fx+ arena arena-bytes)]
           [size (fx+ stacks (fx* thread-stacks thread-stack-bytes))]
           [mem (make-immobile-bytevector size)])
      (zero-range! mem 0 stack) ; a frame is zeroed when it is pushed, the arena as it grows
      (make-ed mem stack (fx+ stack stack-bytes) glue
               (make-shared arena arena (fx+ arena arena-bytes) arena (make-mutex) '() stacks size stacks))))

  ;; A record of ed for another thread: its own stack.
  (define (ed-fork ed stack)
    (make-ed (ed-mem ed) stack (fx+ stack thread-stack-bytes) (ed-glue ed) (ed-shared ed)))

  (define zeros (make-bytevector 65536 0))

  (define (zero-range! mem from to)
    (let loop ([p from])
      (when (fx< p to)
        (let ([n (fxmin 65536 (fx- to p))])
          (bytevector-copy! zeros 0 mem p n)
          (loop (fx+ p n))))))

  ;; host_exit: the condition run catches, the C's longjmp to main.
  (define-condition-type &exit &condition make-exit-condition exit-condition?
    (code exit-condition-code))
  (define (raise-exit code) (raise (make-exit-condition code)))

  ;; n bytes of the arena, 16-aligned as max_align_t is, or #f when it has
  ;; not that many (the host says so and exits, as the C's does).  Several
  ;; threads may allocate at once: the regex engine of the parallel :%s.
  (define (arena-alloc ed n)
    (let ([sh (ed-shared ed)] [want (bitwise-and (+ n 15) (bitwise-not 15))])
      (with-mutex (shared-lock sh)
        (let ([p (shared-next sh)])
          (if (or (< want n) (> want (- (shared-end sh) p)))
              (values #f (fx- p (shared-arena-base sh)))
              (let ([next (fx+ p want)])
                (when (fx> next (shared-zeroed sh))
                  ;; zero ahead, a MiB at a time
                  (let ([z (fxmin (shared-end sh) (fx+ next (fx* 1024 1024)))])
                    (zero-range! (ed-mem ed) (shared-zeroed sh) z)
                    (shared-zeroed-set! sh z)))
                (shared-next-set! sh next)
                (values p (fx- p (shared-arena-base sh)))))))))

  ;; --- memory --------------------------------------------------------------

  ;; The accessors name the bytevector mem of the place they are used at: a
  ;; function of the core binds it once, (ed-mem ed).
  (define-syntax define-mem
    (syntax-rules ()
      [(_ (name arg ...) body)
       (define-mem-in mem (name arg ...) body)]))
  (define-syntax define-mem-in
    (syntax-rules ()
      [(_ m (name arg ...) body)
       (define-syntax name
         (lambda (x)
           (syntax-case x ()
             [(k arg ...)
              (with-syntax ([m (datum->syntax #'k 'mem)])
                #'body)])))]))

  (define-mem-in mem (ld-s8 p) (bytevector-s8-ref mem p))
  (define-mem-in mem (ld-u8 p) (bytevector-u8-ref mem p))
  (define-mem-in mem (ld-s16 p) (bytevector-s16-ref mem p 'little))
  (define-mem-in mem (ld-u16 p) (bytevector-u16-ref mem p 'little))
  (define-mem-in mem (ld-s32 p) (bytevector-s32-ref mem p 'little))
  (define-mem-in mem (ld-u32 p) (bytevector-u32-ref mem p 'little))
  (define-mem-in mem (ld-s64 p) (bytevector-s64-ref mem p 'little))
  (define-mem-in mem (ld-u64 p) (bytevector-u64-ref mem p 'little))
  (define-mem-in mem (ld-ptr p) (bytevector-s64-ref mem p 'little))
  (define-mem-in mem (ld-bool p) (not (fx= 0 (bytevector-u8-ref mem p))))
  (define-mem-in mem (st-s8! p v) (bytevector-s8-set! mem p v))
  (define-mem-in mem (st-u8! p v) (bytevector-u8-set! mem p v))
  (define-mem-in mem (st-s16! p v) (bytevector-s16-set! mem p v 'little))
  (define-mem-in mem (st-u16! p v) (bytevector-u16-set! mem p v 'little))
  (define-mem-in mem (st-s32! p v) (bytevector-s32-set! mem p v 'little))
  (define-mem-in mem (st-u32! p v) (bytevector-u32-set! mem p v 'little))
  (define-mem-in mem (st-s64! p v) (bytevector-s64-set! mem p v 'little))
  (define-mem-in mem (st-u64! p v) (bytevector-u64-set! mem p v 'little))
  (define-mem-in mem (st-ptr! p v) (bytevector-s64-set! mem p v 'little))
  (define-mem-in mem (st-bool! p v) (bytevector-u8-set! mem p (if v 1 0)))
;; (mem-ref kind m p), (mem-set! kind m p v): a load and a store of a
  ;; kind -- s8 to u64, ptr, bool -- at p of the bytevector m.
  (define-syntax mem-ref
    (lambda (x)
      (syntax-case x ()
        [(_ kind m p)
         (case (syntax->datum #'kind)
           [(s8) #'(bytevector-s8-ref m p)]
           [(u8) #'(bytevector-u8-ref m p)]
           [(s16) #'(bytevector-s16-ref m p 'little)]
           [(u16) #'(bytevector-u16-ref m p 'little)]
           [(s32) #'(bytevector-s32-ref m p 'little)]
           [(u32) #'(bytevector-u32-ref m p 'little)]
           [(s64 ptr) #'(bytevector-s64-ref m p 'little)]
           [(u64) #'(bytevector-u64-ref m p 'little)]
           [(bool) #'(not (fx= 0 (bytevector-u8-ref m p)))])])))
  (define-syntax mem-set!
    (lambda (x)
      (syntax-case x ()
        [(_ kind m p v)
         (case (syntax->datum #'kind)
           [(s8) #'(bytevector-s8-set! m p v)]
           [(u8) #'(bytevector-u8-set! m p v)]
           [(s16) #'(bytevector-s16-set! m p v 'little)]
           [(u16) #'(bytevector-u16-set! m p v 'little)]
           [(s32) #'(bytevector-s32-set! m p v 'little)]
           [(u32) #'(bytevector-u32-set! m p v 'little)]
           [(s64 ptr) #'(bytevector-s64-set! m p v 'little)]
           [(u64) #'(bytevector-u64-set! m p v 'little)]
           [(bool) #'(bytevector-u8-set! m p (if v 1 0))])])))

  ;; The C's objects by name, as the generated library defines them; each
  ;; reads and writes the bytevector mem of the place it is used at.
  ;;
  ;; (define-c-object name &name kind addr): a file-scope object at addr.
  ;; Of a scalar kind, name reads it, (set! name v) writes it, &name is its
  ;; address; an array or a struct (kind agg) is its address, as C has it,
  ;; under both names.
  (define-syntax define-c-object
    (lambda (x)
      (syntax-case x ()
        [(_ name addr-name kind addr)
         (eq? (syntax->datum #'kind) 'agg)
         #'(begin
             (define-syntax name (identifier-syntax addr))
             (define-syntax addr-name (identifier-syntax addr)))]
        [(_ name addr-name kind addr)
         #'(begin
             (define-syntax name
               (make-variable-transformer
                 (lambda (y)
                   (syntax-case y (set!)
                     [(set! k e) (with-syntax ([m (datum->syntax #'k 'mem)]) #'(mem-set! kind m addr e))]
                     [k (identifier? #'k) (with-syntax ([m (datum->syntax #'k 'mem)]) #'(mem-ref kind m addr))]))))
             (define-syntax addr-name (identifier-syntax addr)))])))

  ;; (define-c-local name &name kind off): a local of the C's in the call's
  ;; frame, at fr + off; as define-c-object's, the frame and the memory
  ;; those of the function it is in.
  (define-syntax define-c-local
    (lambda (x)
      (syntax-case x ()
        [(_ name addr-name kind off)
         (with-syntax ([fr (datum->syntax #'name 'fr)] [m (datum->syntax #'name 'mem)])
           (if (eq? (syntax->datum #'kind) 'agg)
               #'(begin
                   (define-syntax name (identifier-syntax (fx+ fr off)))
                   (define-syntax addr-name (identifier-syntax (fx+ fr off))))
               #'(begin
                   (define-syntax name
                     (make-variable-transformer
                       (lambda (y)
                         (syntax-case y (set!)
                           [(set! k e) #'(mem-set! kind m (fx+ fr off) e)]
                           [k (identifier? #'k) #'(mem-ref kind m (fx+ fr off))]))))
                   (define-syntax addr-name (identifier-syntax (fx+ fr off))))))])))

  ;; (define-c-member name kind off): a member of a struct, at off from the
  ;; struct's address -- read as a record's field is: (name p) reads it,
  ;; (name-set! p v) writes it, (name& p) is its address; a member that is
  ;; an array or a struct (kind agg) has the address alone.
  (define-syntax define-c-member
    (lambda (x)
      (define (suffix id s)
        (datum->syntax id (string->symbol (string-append (symbol->string (syntax->datum id)) s))))
      (syntax-case x ()
        [(_ name kind off)
         (eq? (syntax->datum #'kind) 'agg)
         (with-syntax ([addr (suffix #'name "&")])
           #'(define-syntax addr (syntax-rules () [(_ p) (fx+ p off)])))]
        [(_ name kind off)
         (with-syntax ([set (suffix #'name "-set!")] [addr (suffix #'name "&")])
           #'(begin
               (define-syntax name
                 (lambda (y)
                   (syntax-case y ()
                     [(k p) (with-syntax ([m (datum->syntax #'k 'mem)]) #'(mem-ref kind m (fx+ p off)))])))
               (define-syntax set
                 (lambda (y)
                   (syntax-case y ()
                     [(k p v) (with-syntax ([m (datum->syntax #'k 'mem)]) #'(mem-set! kind m (fx+ p off) v))])))
               (define-syntax addr (syntax-rules () [(_ p) (fx+ p off)]))))])))

  ;; n bytes from src to dst, overlapping or not: a struct's copy
  (define-mem-in mem (mem-copy! dst src n) (bytevector-copy! mem src mem dst n))
  (define-mem-in mem (mem-zero! p n) (zero-range! mem p (fx+ p n)))
  (define-mem-in mem (mem-fill! p c n) (fill-range! mem p c n))

  ;; n bytes at p set to c: zeros copied, as zero-range! does, else a byte
  ;; at a time
  (define (fill-range! mem p c n)
    (if (fx= c 0)
        (zero-range! mem p (fx+ p n))
        (let loop ([i 0])
          (when (fx< i n)
            (bytevector-u8-set! mem (fx+ p i) c)
            (loop (fx+ i 1))))))

  ;; The bytes of s, a string of characters below 256, written at p: the
  ;; image of the core's initial data.
  (define (mem-image! mem p s)
    (let ([n (string-length s)])
      (let loop ([i 0])
        (when (fx< i n)
          (bytevector-u8-set! mem (fx+ p i) (char->integer (string-ref s i)))
          (loop (fx+ i 1))))))

  ;; The C string at p, as a bytevector; and as a string, a byte a character.
  (define (mem-bytes mem p)
    (let loop ([n 0])
      (if (fx= 0 (bytevector-u8-ref mem (fx+ p n)))
          (let ([b (make-bytevector n)])
            (bytevector-copy! mem p b 0 n)
            b)
          (loop (fx+ n 1)))))
  (define (mem-string mem p)
    (let ([b (mem-bytes mem p)])
      (let ([s (make-string (bytevector-length b))])
        (let loop ([i 0])
          (if (fx= i (bytevector-length b))
              s
              (begin (string-set! s i (integer->char (bytevector-u8-ref b i))) (loop (fx+ i 1))))))))

;; (define-c-enum name value): a named constant of the C's, whose value
  ;; c-case reads when it expands.
  (define c-enum)
  (define-syntax define-c-enum
    (syntax-rules ()
      [(_ name v) (begin (define name v) (define-property name c-enum 'v))]))

  ;; (c-case e [(label ...) body ...] ... [else body ...]): a case whose
  ;; labels are as the C spells them -- a constant's name, a character
  ;; (its code), a number -- each its value at expansion: the case it
  ;; expands to is one of numbers.
  (define-syntax c-case
    (lambda (x)
      (lambda (lookup)
        (define (value l)
          (let ([d (syntax->datum l)])
            (cond
              [(identifier? l)
               (or (lookup l #'c-enum) (syntax-violation 'c-case "not a constant of the C's" x l))]
              [(char? d) (char->integer d)]
              [else d])))
        (define (clause c)
          (syntax-case c (else)
            [(else b ...) c]
            [((l ...) b ...) (with-syntax ([(v ...) (map value #'(l ...))]) #'((v ...) b ...))]))
        (syntax-case x ()
          [(_ e c ...) (with-syntax ([(c2 ...) (map clause #'(c ...))]) #'(case e c2 ...))]))))

;; A C character constant: (ch #\a) is the integer of its code, at
  ;; expansion.
  (define-syntax ch
    (lambda (x)
      (syntax-case x ()
        [(_ c) (char? (syntax->datum #'c)) (char->integer (syntax->datum #'c))])))

  ;; A string literal: its address in the image, the text beside it.
  (define-syntax c-str (syntax-rules () [(_ addr text) addr]))

  ;; --- a call's frame ---------------------------------------------------------

  ;; The locals of a call that live in memory: n bytes (a multiple of 16) of
  ;; the thread's stack, zeroed; frame-pop! gives them back where the call
  ;; returns.
  (define-syntax frame-push!
    (syntax-rules ()
      [(_ ed n)
       (let* ([e ed] [fr (ed-sp e)] [sp (fx+ fr n)])
         (when (fx> sp (ed-limit e)) (stack-overflow))
         (ed-sp-set! e sp)
         (zero-range! (ed-mem e) fr sp)
         fr)]))
  (define-syntax frame-pop!
    (syntax-rules () [(_ ed fr) (ed-sp-set! ed fr)]))
  (define (stack-overflow) (error 'whimsical "the stack of call frames overflowed"))

  ;; --- C's integers ---------------------------------------------------------

  (define (slow-wrap x bits signed)
    (let* ([m (bitwise-arithmetic-shift-left 1 bits)]
           [y (bitwise-and x (- m 1))])
      (if (and signed (>= y (bitwise-arithmetic-shift-right m 1))) (- y m) y)))

  ;; an exact integer converted to a C type: its bits, sign- or zero-extended
  (define-syntax ->i8
    (syntax-rules () [(_ e) (let ([x e]) (if (fixnum? x) (fxsra (fxsll/wraparound x 53) 53) (slow-wrap x 8 #t)))]))
  (define-syntax ->u8
    (syntax-rules () [(_ e) (let ([x e]) (if (fixnum? x) (fxand x #xff) (slow-wrap x 8 #f)))]))
  (define-syntax ->i16
    (syntax-rules () [(_ e) (let ([x e]) (if (fixnum? x) (fxsra (fxsll/wraparound x 45) 45) (slow-wrap x 16 #t)))]))
  (define-syntax ->u16
    (syntax-rules () [(_ e) (let ([x e]) (if (fixnum? x) (fxand x #xffff) (slow-wrap x 16 #f)))]))
  (define-syntax ->i32
    (syntax-rules () [(_ e) (let ([x e]) (if (fixnum? x) (fxsra (fxsll/wraparound x 29) 29) (slow-wrap x 32 #t)))]))
  (define-syntax ->u32
    (syntax-rules () [(_ e) (let ([x e]) (if (fixnum? x) (fxand x #xffffffff) (slow-wrap x 32 #f)))]))
  (define-syntax ->i64
    (syntax-rules () [(_ e) (let ([x e]) (if (fixnum? x) x (slow-wrap x 64 #t)))]))
  (define-syntax ->u64
    (syntax-rules () [(_ e) (let ([x e]) (if (and (fixnum? x) (fx>= x 0)) x (slow-wrap x 64 #f)))]))
  (define-syntax b->i (syntax-rules () [(_ b) (if b 1 0)]))

  ;; int: fixnum arithmetic, wrapped to 32 bits
  (define-syntax wrap32 (syntax-rules () [(_ e) (fxsra (fxsll/wraparound e 29) 29)]))
  (define-syntax i32+ (syntax-rules () [(_ a b) (wrap32 (fx+ a b))]))
  (define-syntax i32- (syntax-rules () [(_ a b) (wrap32 (fx- a b))]))
  (define-syntax i32* (syntax-rules () [(_ a b) (wrap32 (fx*/wraparound a b))]))
  (define-syntax i32/ (syntax-rules () [(_ a b) (wrap32 (fxquotient a b))]))
  (define-syntax i32% (syntax-rules () [(_ a b) (fxremainder a b)]))
  (define-syntax i32<< (syntax-rules () [(_ a n) (wrap32 (fxsll/wraparound a n))]))
  (define-syntax i32>> (syntax-rules () [(_ a n) (fxsra a n)]))
  ;; unsigned int
  (define-syntax u32+ (syntax-rules () [(_ a b) (fxand (fx+ a b) #xffffffff)]))
  (define-syntax u32- (syntax-rules () [(_ a b) (fxand (fx- a b) #xffffffff)]))
  (define-syntax u32* (syntax-rules () [(_ a b) (fxand (fx*/wraparound a b) #xffffffff)]))
  (define-syntax u32/ (syntax-rules () [(_ a b) (fxquotient a b)]))
  (define-syntax u32% (syntax-rules () [(_ a b) (fxremainder a b)]))
  (define-syntax u32<< (syntax-rules () [(_ a n) (fxand (fxsll/wraparound a n) #xffffffff)]))
  (define-syntax u32>> (syntax-rules () [(_ a n) (fxsrl a n)]))
  (define-syntax u32~ (syntax-rules () [(_ a) (fxxor a #xffffffff)]))
  ;; long: generic arithmetic, a fixnum test, and the wrap past 2^60
  (define-syntax i64+ (syntax-rules () [(_ a b) (->i64 (+ a b))]))
  (define-syntax i64- (syntax-rules () [(_ a b) (->i64 (- a b))]))
  (define-syntax i64* (syntax-rules () [(_ a b) (->i64 (* a b))]))
  (define-syntax i64/ (syntax-rules () [(_ a b) (->i64 (quotient a b))]))
  (define-syntax i64% (syntax-rules () [(_ a b) (remainder a b)]))
  (define-syntax i64<< (syntax-rules () [(_ a n) (->i64 (bitwise-arithmetic-shift-left a n))]))
  (define-syntax i64>> (syntax-rules () [(_ a n) (bitwise-arithmetic-shift-right a n)]))
  ;; unsigned long
  (define-syntax u64+ (syntax-rules () [(_ a b) (->u64 (+ a b))]))
  (define-syntax u64- (syntax-rules () [(_ a b) (->u64 (- a b))]))
  (define-syntax u64* (syntax-rules () [(_ a b) (->u64 (* a b))]))
  (define-syntax u64/ (syntax-rules () [(_ a b) (quotient a b)]))
  (define-syntax u64% (syntax-rules () [(_ a b) (remainder a b)]))
  (define-syntax u64<< (syntax-rules () [(_ a n) (->u64 (bitwise-arithmetic-shift-left a n))]))
  (define-syntax u64>> (syntax-rules () [(_ a n) (bitwise-arithmetic-shift-right a n)]))
  (define-syntax u64~ (syntax-rules () [(_ a) (- #xffffffffffffffff a)]))

  ;; --- function pointers ------------------------------------------------------

  ;; A function's address: its index in the core's table, in the null page,
  ;; where no object is.
  (define-syntax fn-ptr (syntax-rules () [(_ i) (fx* (fx+ i 1) 16)]))
  (define-syntax fn-index (syntax-rules () [(_ p) (fx- (fxsrl p 4) 1)]))

  ;; --- a loop over lines, in parallel -------------------------------------------

  ;; work over [0, n) in chunks at once, and whether every chunk's work did:
  ;; the parallel body of a function the C writes as one loop over a range
  ;; (match_lines, as editor/chunks.go's Chunks).  About four chunks a
  ;; worker, none smaller than 64, on as many workers as the machine has
  ;; processors, each a thread with a stack of its own; a range of one
  ;; chunk, or one worker, runs on the caller's thread, and a chunk that did
  ;; not stops the chunks not yet started.  (work ed from to) must be the
  ;; loop's over its part and write nothing another part reads.
  (define (chunks ed n work)
    (let* ([workers (processors)]
           [w (fx* 4 workers)]
           [size (fxmax 64 (fxquotient (fx+ n (fx- w 1)) w))])
      (if (or (fx= workers 1) (fx>= size n))
          (work ed 0 n)
          (let* ([count (fxquotient (fx+ n (fx- size 1)) size)]
                 [lock (make-mutex)]
                 [next 0]
                 [failed #f]
                 [error #f]
                 [take (lambda ()
                         (with-mutex lock
                           (if (or failed error (fx>= next count))
                               #f
                               (let ([i next]) (set! next (fx+ next 1)) i))))]
                 [run (lambda (ted)
                        (let loop ()
                          (let ([i (take)])
                            (when i
                              (let ([from (fx* i size)])
                                (guard (c [#t (with-mutex lock (unless error (set! error c)))])
                                  (unless (work ted from (fxmin n (fx+ from size)))
                                    (with-mutex lock (set! failed #t))))
                                (loop))))))]
                 [stacks (map (lambda (i) (take-stack ed)) (iota (fxmin workers count)))]
                 [threads (map (lambda (s) (fork-thread (lambda () (run (ed-fork ed s))))) stacks)])
            (for-each thread-join threads)
            (for-each (lambda (s) (give-stack ed s)) stacks)
            (when error (raise error))
            (not failed)))))

  (define (iota n) (let loop ([i (fx- n 1)] [acc '()]) (if (fx< i 0) acc (loop (fx- i 1) (cons i acc)))))

  ;; a worker's stack: one given back, or a new one of the region's
  (define (take-stack ed)
    (let ([sh (ed-shared ed)])
      (with-mutex (shared-lock sh)
        (let ([free (shared-stacks sh)])
          (if (pair? free)
              (begin (shared-stacks-set! sh (cdr free)) (car free))
              (let ([s (shared-stack-next sh)])
                (when (fx> (fx+ s thread-stack-bytes) (shared-stack-end sh))
                  (error 'whimsical "no stack left for another worker"))
                (shared-stack-next-set! sh (fx+ s thread-stack-bytes))
                s))))))
  (define (give-stack ed s)
    (let ([sh (ed-shared ed)])
      (with-mutex (shared-lock sh)
        (shared-stacks-set! sh (cons s (shared-stacks sh))))))

  ;; the processors the machine has, counted once -- or WHIMSICAL_WORKERS,
  ;; when the environment says how many workers to use (1: the C's loop,
  ;; on the caller's thread)
  (define nproc #f)
  (define (processors)
    (or nproc
        (let* ([env (getenv "WHIMSICAL_WORKERS")]
               [n (or (and env (string->number env))
                      (guard (c [#t 1])
                        (call-with-input-file "/proc/cpuinfo"
                          (lambda (p)
                            (let loop ([n 0])
                              (let ([l (get-line p)])
                                (cond
                                  [(eof-object? l) n]
                                  [(and (fx>= (string-length l) 9) (string=? (substring l 0 9) "processor")) (loop (fx+ n 1))]
                                  [else (loop n)])))))))])
          (set! nproc (fxmin thread-stacks (fxmax 1 n)))
          nproc)))
)
