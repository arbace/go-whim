(ns whim.cljhost
  "The C's host functions for the editor in Clojure: braaam/Whim.java's glue,
  as functions of the editor.  The generated core (whim.editor, written by
  crefactor/togo's Clojure backend) calls each as the C calls it --
  (whim.cljhost/host_write ed p n) -- and each is a line of glue to the
  whim.host.Host the editor was made on, (whim.editor/host-of ed): the Java
  editor's interface and its terminal host, used through interop unchanged.
  vim's printf is whim.host.Printf, handed the core's functions it needs.

  The two namespaces do not require each other: whim.editor requires this one,
  and this one reaches the core -- host-of, deathtrap, and what the printf
  needs -- through requiring-resolve, once each.  doc/CLOJURE.md, the
  contract, says what either side may assume."
  (:import (java.util Collections Map WeakHashMap)
           (java.util.function IntConsumer)
           (whim.host Host Host$TtyKeys Printf Printf$Core)
           (whim.rt BytePtr IntPtr)))

(set! *warn-on-reflection* true)
(set! *unchecked-math* :warn-on-boxed)

;; --- the core, reached by name

(defn- core-fn
  "The core's function sym, resolved once, when first called: the core
  requires this namespace, so neither can name the other at compile time."
  [sym]
  (let [f (delay (or (requiring-resolve sym)
                     (throw (IllegalStateException. (str "whim.cljhost: the core has no " sym)))))]
    (fn
      ([ed] ((deref f) ed))
      ([ed a] ((deref f) ed a)))))

(def ^:private host-of* (core-fn 'whim.editor/host-of))
(def ^:private deathtrap* (core-fn 'whim.editor/deathtrap))
(def ^:private gettext_* (core-fn 'whim.editor/gettext_))
(def ^:private emsg* (core-fn 'whim.editor/emsg))
(def ^:private iemsg* (core-fn 'whim.editor/iemsg))
(def ^:private emsg_iobuff_room* (core-fn 'whim.editor/emsg_iobuff_room))
(def ^:private iobuff_or* (core-fn 'whim.editor/iobuff_or))
(def ^:private utfc_ptr2len* (core-fn 'whim.editor/utfc_ptr2len))
(def ^:private utf_ptr2cells* (core-fn 'whim.editor/utf_ptr2cells))

(defn- bytes-ptr
  "A byte array or a BytePtr, as the BytePtr it decays to."
  ^BytePtr [x]
  (if (instance? BytePtr x) x (BytePtr. ^bytes x 0)))

(defn- core-object
  "The core's file-scope object sym on ed: whim.editor/<sym> is a function of
  the editor that gives it, or -- for a constant the editors share -- the
  value itself.  Only the printf's error paths read one (IObuff and
  e_val_too_large)."
  [sym ed]
  (let [v (deref (or (requiring-resolve sym)
                     (throw (IllegalStateException. (str "whim.cljhost: the core has no " sym)))))]
    (if (fn? v) (v ed) v)))

(defn- host
  "The Host the editor ed was made on."
  ^Host [ed]
  (host-of* ed))

;; --- the glue: the host functions the core calls, the editor first, in the
;; C's argument types (every integer a long, a pointer the runtime's class)

(defn musl_host_init
  "Start catching the signals: SIGHUP and SIGTERM call the core's deathtrap
  on the thread running the core."
  [ed]
  (.init (host ed) (reify IntConsumer
                     (accept [_ sig] (deathtrap* ed (long sig)))))
  nil)

(defn musl_get_winsize
  "The terminal's size into rows and cols: OK (1), or FAIL (0) when it has none."
  ^long [ed ^IntPtr rows ^IntPtr cols]
  (if-let [^ints ws (.winSize (host ed))]
    (do (.put rows (aget ws 0))
        (.put cols (aget ws 1))
        1)
    0))

(defn musl_term_start [ed]
  (.termStart (host ed))
  nil)

(defn musl_term_stop [ed]
  (.termStop (host ed))
  nil)

(defn musl_tty_keys
  "The terminal on fd's erase and interrupt characters, and whether it maps CR
  to NL on input and NL to CR-NL on output: OK, or FAIL when fd is none."
  [ed fd ^IntPtr bs ^IntPtr intr ^IntPtr cr ^IntPtr nlcr]
  (if-let [^Host$TtyKeys k (.ttyKeys (host ed) (int fd))]
    (do (.put bs (.erase k))
        (.put intr (.intr k))
        (.put cr (if (.icrnl k) 1 0))
        (.put nlcr (if (.onlcr k) 1 0))
        1)
    0))

(defn musl_now_ms ^long [ed]
  (.nowMs (host ed)))

(defn host_time ^long [ed]
  (.time (host ed)))

(defn musl_delay [ed ^long ms ^long interruptible]
  (.delay (host ed) ms (not (zero? interruptible)))
  nil)

(defn musl_wait_for_input ^long [ed ^long ms]
  (if (.waitForInput (host ed) ms) 1 0))

(defn musl_read_input
  "Read up to len bytes into buf.  The host gets no room for a negative
  length, and -1 is the answer for it, as read(2) of (size_t)len gives; the
  host still takes the signals it reads as input, as the C did before its
  read."
  ^long [ed ^BytePtr buf ^long len]
  (let [n (.readInput (host ed) (.-a buf) (.-i buf) (int (max len 0)))]
    (if (neg? len) -1 n)))

(defn host_raise [ed ^long sig]
  (.raise (host ed) (int sig))
  nil)

(defn musl_suspend [ed]
  (.suspend (host ed))
  nil)

(defn host_exit
  "End the editor with r: the terminal host ends the process, a host that
  must not throws whim.host.Exit, which whim.cljmain/run catches."
  [ed ^long r]
  (.exit (host ed) (int r))
  nil)

(defn host_message
  "Write msg, len bytes of it or up to its NUL when len is negative, to the
  error stream when err."
  [ed ^BytePtr msg ^long len ^long err]
  (let [n (if (neg? len) (BytePtr/strlen msg) (int len))]
    (.message (host ed) (.-a msg) (.-i msg) n (not (zero? err))))
  nil)

(defn host_write ^long [ed ^BytePtr s ^long len]
  (cond
    (neg? len) -1
    (zero? len) 0
    :else (.write (host ed) (.-a s) (.-i s) (int len))))

;; The C host allocates from a static 1 GiB arena and never frees; the
;; garbage collector is the allocator here, but the arena's accounting (and
;; its exhaustion message) are kept, per editor.  An editor is on a host of
;; its own, so the count is kept by the host, in a map that does not keep the
;; host alive; the count refers to neither, so the map lets both go.

(def ^:const host-arena-bytes (* 1024 1024 1024))

(def ^:private ^Map arenas (Collections/synchronizedMap (WeakHashMap.)))

(defn- arena
  "The one-element array holding the bytes ed's host has handed out."
  ^longs [ed]
  (let [h (host ed)]
    (or (.get arenas h)
        (let [a (long-array 1)]
          (.put arenas h a)
          a))))

(defn- host-arena-exhausted [ed ^long used ^long n]
  (let [m (str "whim-vim: host arena exhausted: " (Long/toUnsignedString host-arena-bytes) " bytes, "
               (Long/toUnsignedString used) " used, request " (Long/toUnsignedString n) "\n")
        b (BytePtr/alloc (inc (count m)))]
    (dotimes [i (count m)]
      (.set b (int i) (byte (int (.charAt m i)))))
    (host_message ed b (count m) 1)
    (host_exit ed 1)))

(defn host_alloc
  "n zeroed bytes, a BytePtr as Object: the storage a C allocation is."
  [ed ^long n]
  (let [a (arena ed)
        used (aget a 0)
        want (bit-and (+ n 15) (bit-not 15))] ; alignof(max_align_t) is 16
    (when (or (neg? (Long/compareUnsigned want n))
              (pos? (Long/compareUnsigned want (- host-arena-bytes used))))
      (host-arena-exhausted ed used n))
    (aset a 0 (+ used want))
    (BytePtr/alloc n)))

;; --- vim_snprintf, whim.host.Printf given what it needs of the core

(defn- printf-core
  "What the formatter needs of the editor ed: Printf.Core, by the core's
  functions."
  ^Printf$Core [ed]
  (reify Printf$Core
    (gettext [_ msg] (gettext_* ed msg))
    (error [_ msg] (emsg* ed msg) nil)
    (internalError [_ msg] (iemsg* ed msg) nil)
    (iobuff [_] (bytes-ptr (core-object 'whim.editor/IObuff ed)))
    (emsgIobuffRoom [_] (long (emsg_iobuff_room* ed)))
    (iobuffOr [_ s] (iobuff_or* ed s))
    (eValTooLarge [_] (bytes-ptr (core-object 'whim.editor/e_val_too_large ed)))
    (utfcPtr2len [_ p] (int (utfc_ptr2len* ed p)))
    (utfPtr2cells [_ p] (int (utf_ptr2cells* ed p)))))

(defn vim_snprintf
  "vim's printf into str, at most str_m bytes of it: the length the whole
  would have.  args is the variadic arguments as one Object array, boxed as
  the Java editor boxes them (whim.host.Printf says how each is read)."
  [ed ^BytePtr str str_m ^BytePtr fmt ^objects args]
  (.snprintf (Printf. (printf-core ed)) str (long str_m) fmt args))
