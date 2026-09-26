(ns whim.cljmain
  "The editor in Clojure as a program: the generated core (whim.editor) on
  the terminal host, as bin/whim is the Go editor and braaam/Whim.java's
  main the Java one.  run is the editor on any host; -main is the launcher's
  entry, AOT-compiled with :gen-class."
  (:require [whim.cljhost]
            [whim.editor :as ed])
  (:import (java.nio.charset Charset StandardCharsets)
           (java.nio.file Files Path)
           (java.util Arrays)
           (whim.host Exit Host Term)
           (whim.rt BytePtr Ptr))
  (:gen-class))

(set! *warn-on-reflection* true)
(set! *unchecked-math* :warn-on-boxed)

(defn run
  "Run a new editor on h with the command line args -- a sequence of byte
  arrays, args[0] the program's name -- and return its exit status: vim_main's,
  or the code of a whim.host.Exit thrown by h.  The arguments are bytes, as
  C's are."
  ^long [^Host h args]
  (try
    (let [^objects argv (object-array (inc (count args)))]
      (dorun (map-indexed (fn [i ^bytes a]
                            (aset argv (int i) (BytePtr. (Arrays/copyOf a (inc (alength a))) 0)))
                          args))
      (long (ed/vim_main (ed/new-editor h) (count args) (Ptr. argv 0))))
    (catch Exit e
      (long (.-code e)))))

(defn- native-charset
  "The charset the JVM decoded the command line with."
  ^Charset []
  (let [n (System/getProperty "sun.jnu.encoding")]
    (try
      (if n (Charset/forName n) StandardCharsets/UTF_8)
      (catch RuntimeException _ StandardCharsets/UTF_8))))

(defn- split-nul
  "The NUL-terminated strings of c, as byte arrays."
  [^bytes c]
  (loop [i 0, from 0, out (transient [])]
    (cond
      (= i (alength c)) (persistent! out)
      (zero? (aget c i)) (recur (inc i) (inc i) (conj! out (Arrays/copyOfRange c from i)))
      :else (recur (inc i) from out))))

(defn arg-bytes
  "The bytes of the arguments Java decoded into args: the last (count args)
  entries of /proc/self/cmdline, which are exactly what the launcher was
  handed -- a decoding by the platform's charset is not, for bytes it cannot
  decode.  Encoded again from the strings when /proc says something else
  (another system, or a count that does not match): Whim.argBytes."
  [args]
  (let [cs (native-charset)
        n (count args)
        again (fn [] (mapv (fn [^String s] (.getBytes s cs)) args))
        all (try (split-nul (Files/readAllBytes (Path/of "/proc/self/cmdline" (make-array String 0))))
                 (catch java.io.IOException _ nil)
                 (catch RuntimeException _ nil))]
    (if (and all (>= (count all) n))
      (let [tail (subvec all (- (count all) n))]
        (if (every? true? (map (fn [^bytes b ^String s] (= (String. b cs) s)) tail args))
          tail
          (again)))
      (again))))

(defn- report
  "The editor failed: say where, as a crash would -- the exception and the
  first frames, braaam's report."
  [^Throwable t]
  (binding [*out* *err*]
    (println (str "vijure: " t))
    (doseq [^StackTraceElement f (take 24 (.getStackTrace t))]
      (println (str "\tat " f)))
    (flush)))

(defn -main
  "The launcher: the editor on the terminal.  The program's name is the
  system property whim.argv0 (the launcher script's $0), since Java hands
  main none.  The core runs on a thread of its own with a stack as large as a
  C process's could grow: vim recurses, and a Clojure frame is larger than a
  compiled C one."
  [& args]
  (let [argv (into [(.getBytes (System/getProperty "whim.argv0" "vijure") (native-charset))]
                   (arg-bytes (vec args)))
        status (int-array 1)
        core (Thread. nil
                      (fn []
                        (try
                          (aset status 0 (int (run (Term.) argv)))
                          (catch Throwable t
                            (report t)
                            (aset status 0 70))))
                      "whim"
                      (bit-shift-left 1 30))]
    (.start core)
    (.join core)
    (System/exit (aget status 0))))
