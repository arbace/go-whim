(ns whim.editor
  "A STAND-IN for the generated core, for vijure's and the suite's tests:
  it provides what doc/CLOJURE.md's contract says the generated namespace
  provides -- new-editor, host-of, vim_main and the core functions the glue
  reaches by name -- and calls the host functions (whim.cljhost) as the core
  does, so that the glue, the launcher and the build are proven before the
  real namespace exists.  It is no editor: vim_main reports what the host
  answers and acts on a few keys."
  (:require [whim.cljhost :as h])
  (:import (whim.host Host)
           (whim.rt BytePtr IntPtr Ptr)))

(set! *warn-on-reflection* true)
(set! *unchecked-math* :warn-on-boxed)

(deftype Editor [^Host host ^BytePtr iobuff])

(defn new-editor [^Host host]
  (Editor. host (BytePtr/alloc 1025)))

(defn host-of ^Host [^Editor ed]
  (.-host ed))

;; the core functions and objects the glue reaches (whim.cljhost's printf-core)
(defn IObuff [^Editor ed] (.-iobuff ed))
(def e_val_too_large (.getBytes "E1510: Value too large: %s\u0000" "US-ASCII"))
(defn gettext_ [ed msg] msg)
(defn emsg [ed ^BytePtr s]
  (h/host_message ed s -1 1)
  (h/host_message ed (BytePtr/lit "\n") 1 1)
  1)
(defn iemsg [ed s] (emsg ed s) nil)
(defn emsg_iobuff_room ^long [ed] 1025)
(defn iobuff_or [ed s] (IObuff ed))
(defn utfc_ptr2len ^long [ed p] 1)
(defn utf_ptr2cells ^long [ed p] 1)
(defn deathtrap [ed sig]
  (emsg ed (BytePtr/lit (str "deathtrap " sig)))
  (h/host_exit ed 1))

(defn- say
  "Format fmt with args through vim_snprintf and write it to the screen."
  [ed ^String fmt & args]
  (let [buf (BytePtr/alloc 200)
        n (h/vim_snprintf ed buf 200 (BytePtr/lit fmt) (object-array args))]
    (h/host_write ed buf (min (long n) 199))))

(defn vim_main
  "Report what the host answers, then read the keys: i writes \" INSERT\", T
  throws, P provokes a printf error (through emsg), A exhausts the arena, E
  exits with 3 through the host, V writes argv[0] (which the other arguments
  are written without: the suite runs an editor and its control under two
  names), q returns 5 from vim_main; the end of input returns 0."
  [ed argc ^Ptr argv]
  (h/musl_host_init ed)
  (say ed "argc=%d" (int argc))
  (doseq [i (range 1 argc)]
    (say ed " [%s]" (.at argv (int i))))
  (say ed "\r\n")
  (let [rows (IntPtr/alloc 1) cols (IntPtr/alloc 1)]
    (say ed "winsize=%d " (int (h/musl_get_winsize ed rows cols))))
  (let [bs (IntPtr/alloc 1) intr (IntPtr/alloc 1) cr (IntPtr/alloc 1) nl (IntPtr/alloc 1)]
    (say ed "ttykeys=%d " (int (h/musl_tty_keys ed 0 bs intr cr nl))))
  (let [^BytePtr p (h/host_alloc ed 100)]
    (say ed "alloc=%d zero=%d " (int (.len p)) (int (.at p 99))))
  (say ed "time=%d now=%d\r\n" (Boolean/valueOf (pos? (h/host_time ed))) (Boolean/valueOf (>= (h/musl_now_ms ed) 0)))
  (say ed "%s=%ld %5.2s|%-4d|%x|%c\r\n" (BytePtr/lit "fmt") (long -7) (BytePtr/lit "abc") (int 42) (int 255) (byte 65))
  (h/musl_term_start ed)
  (let [buf (BytePtr/alloc 64)]
    (loop []
      (h/musl_wait_for_input ed -1)
      (let [n (h/musl_read_input ed buf 64)]
        (if (<= n 0)
          (do (h/musl_term_stop ed) 0)
          (let [r (loop [i 0]
                    (if (= i n)
                      nil
                      (case (char (bit-and (.at buf (int i)) 0xff))
                        \i (do (h/host_write ed (BytePtr/lit " INSERT") 7) (recur (inc i)))
                        \T (throw (IllegalStateException. "the stand-in was told to throw"))
                        \P (do (say ed "%2$s %1$s %1$d" (BytePtr/lit "x") (BytePtr/lit "y")) (recur (inc i)))
                        \A (do (h/host_alloc ed (bit-shift-left 1 31)) (recur (inc i)))
                        \E (do (h/host_exit ed 3) (recur (inc i)))
                        \V (do (say ed "[%s]" (.at argv 0)) (recur (inc i)))
                        \q 5
                        (recur (inc i)))))]
            (if r
              (do (h/musl_term_stop ed) r)
              (recur))))))))
