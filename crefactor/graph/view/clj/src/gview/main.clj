;; gview.main: `go tool whim view`'s command line over a graph's EDN.
;;
;;   view-clj [--ids] [--depth N] [--show node|stmt|fn|none] [--stop HEADS] [--time]
;;            callers F | callees F | uses NAME | member S.M | type T | def NAME
;;            | follow 'STEPS' ROOT   FILE.edn
;;   view-clj --batch CASES OUTDIR FILE.edn
;;   view-clj --serve FILE.edn
;;
;; FILE.edn is what `go tool whim graph --edn` writes.  The output is what
;; `go tool whim view` prints for the same arguments on the graph the EDN
;; was written from.  --batch reads the EDN once and runs every line of
;; CASES -- a view's arguments, tab-separated (or space-separated when the
;; line holds no tab) -- writing line N's output to OUTDIR/N.out and its
;; error to OUTDIR/N.err.  --serve reads the EDN once and answers requests
;; on stdin, a line each (THE SERVER, below).  --c (def's C view) is the
;; Go's alone: it wants C's printer.
(ns gview.main
  (:require [clojure.java.io :as io]
            [clojure.string :as str]
            [gview.graph :as g]
            [gview.view :as v])
  (:gen-class))

(set! *warn-on-reflection* true)

(def ^:private usage-text
  "usage: view-clj [--ids] [--depth N] [--show node|stmt|fn|none] [--stop HEADS] [--time]\n                callers F | callees F | uses NAME | member S.M | type T | def NAME | follow 'STEPS' ROOT  FILE.edn\n       view-clj --batch CASES OUTDIR FILE.edn\n       view-clj --serve FILE.edn")

(defn- fail [msg] (throw (ex-info msg {:view true})))

(defn parse-args
  "A view's arguments: {:o options :ids :time :pos [VIEW ARG ...]}, or nil."
  [argv]
  (loop [as (seq argv) o {} ids false timing false pos []]
    (if-let [a (first as)]
      (case a
        "--ids" (recur (next as) o true timing pos)
        "--time" (recur (next as) o ids true pos)
        "--depth" (when-let [n (some-> (second as) parse-long)]
                    (recur (nnext as) (assoc o :depth (if (zero? n) -1 n)) ids timing pos))
        "--show" (when-let [s ({"node" :node "stmt" :stmt "fn" :fn "none" :none} (second as))]
                   (recur (nnext as) (assoc o :show s) ids timing pos))
        "--stop" (when (second as)
                   (recur (nnext as) (assoc o :stop (str/split (second as) #",")) ids timing pos))
        (recur (next as) o ids timing (conj pos a)))
      (let [need (if (= (first pos) "follow") 3 2)]
        (when (= (count pos) (inc need))
          {:o o :ids ids :time timing :pos pos :need need})))))

(defn- member-node? [ix n]
  (let [p (g/parent ix n)]
    (or (g/is? n "member") (and p (g/type-def? p) (or (g/is? p "struct") (g/is? p "union"))))))

(defn view-text
  "The text a view prints, on an index; throws ex-info {:view true} on an
  error a user made."
  [ix {:keys [o ids pos need]}]
  (let [vname (first pos)
        arg (pos (dec need))
        roots (g/find-roots ix arg)
        out (StringBuilder.)]
    (when (and (= vname "member") (not (member-node? ix (first roots))))
      (fail (str arg " is not a member: member is S.M")))
    (doseq [root roots]
      (.append out
               ^String (case vname
                         "callers" (v/tree-text ids ix (v/callers ix root o))
                         "callees" (v/tree-text ids ix (v/callees ix root o))
                         "uses" (v/tree-text ids ix (v/uses ix root o))
                         "member" (v/tree-text ids ix (v/member ix root o))
                         "type" (v/tree-text ids ix (v/type-view ix (g/aggregate ix (pos 1)) o))
                         "def" (v/form-text ids (v/def-of ix root))
                         "follow" (v/tree-text ids ix (v/follow ix root (pos 1) o))
                         (throw (ex-info "usage" {:usage true})))))
    (str out)))

(defn- ms [^long t0] (quot (- (System/nanoTime) t0) 1000000))

(defn- load-index [file]
  (let [t0 (System/nanoTime)
        graph (g/read-edn file)
        loaded (ms t0)
        t1 (System/nanoTime)
        ix (g/new-index graph)]
    [ix loaded (ms t1)]))

(defn- err [& xs] (binding [*out* *err*] (println (apply str xs))))

(defn- batch [cases outdir file]
  (let [[ix loaded indexed] (load-index file)
        t0 (System/nanoTime)
        lines (remove str/blank? (str/split-lines (slurp cases)))]
    (.mkdirs (io/file outdir))
    (doseq [[i line] (map-indexed vector lines)]
      (let [a (parse-args (concat (str/split line (if (str/includes? line "\t") #"\t" #"\s+")) [file]))
            [text e] (try (if a [(view-text ix a) nil] [nil usage-text])
                          (catch clojure.lang.ExceptionInfo x
                            (if (:view (ex-data x)) [nil (ex-message x)] [nil usage-text])))]
        (spit (io/file outdir (str (inc i) ".out")) (or text ""))
        (spit (io/file outdir (str (inc i) ".err")) (if e (str "  view-clj     " e "\n") ""))))
    (err "  view-clj     read " loaded "ms, indexed " indexed "ms, " (count lines) " views built and printed " (ms t0) "ms")
    0))

;; THE SERVER.  `view-clj --serve FILE.edn`: the graph read and indexed
;; once, then a request a line on stdin, each answered on stdout by a
;; header, a body and an end line:
;;
;;   ok N            N bytes of the answer (a view's text, stats, reload)
;;   error N         N bytes: the message and a newline
;;   ;;end           on a line of its own, after the body
;;
;; N counts the body's bytes (UTF-8) and is exact; ";;end" is for a client
;; that reads lines (a newline goes before it when the body lacks a last
;; one).  A request is a view's arguments without the file --
;; tab-separated, or, with no tab, separated by blanks with '...' or "..."
;; quoting -- or `stats`, `reload [FILE.edn]` (the EDN read again, or
;; another, and the cache emptied), `ping`, `quit`.  End of input ends the
;; server.  Views are cached by their parsed arguments.

(defn split-request
  "A request line's words: tab-separated when it holds a tab, else
  blank-separated with '...' and \"...\" quoting."
  [^String line]
  (if (str/includes? line "\t")
    (vec (remove str/blank? (str/split line #"\t")))
    (let [n (.length line)
          sb (fn [cur] (or cur (StringBuilder.)))]
      (loop [i 0, cur nil, q nil, out []]
        (if (= i n)
          (if cur (conj out (str cur)) out)
          (let [c (.charAt line i)]
            (cond
              q (if (= c (char q))
                  (recur (inc i) (sb cur) nil out)
                  (recur (inc i) (doto ^StringBuilder (sb cur) (.append c)) q out))
              (or (= c \') (= c \")) (recur (inc i) (sb cur) c out)
              (Character/isWhitespace c) (recur (inc i) nil nil (if cur (conj out (str cur)) out))
              :else (recur (inc i) (doto ^StringBuilder (sb cur) (.append c)) nil out))))))))

(defn- node-count ^long [ix]
  (let [^objects a (:by-id ix)]
    (areduce a i c 0 (if (aget a i) (inc c) c))))

(defn open
  "A server's state over FILE.edn: the graph read and indexed once."
  [file]
  (let [[ix loaded indexed] (load-index file)]
    {:file file :ix ix :read-ms loaded :index-ms indexed :nodes (node-count ix)
     :cache (java.util.HashMap.) :bytes 0 :hits 0 :misses 0 :requests 0
     :started (System/nanoTime)}))

(def ^:private cache-limit
  "The cache's characters before it is emptied: a view is rebuilt in ms."
  (* 256 1024 1024))

(defn- stats-text [st]
  (str "file " (:file st) "\n"
       "read " (:read-ms st) " ms, indexed " (:index-ms st) " ms\n"
       "nodes " (:nodes st) "\n"
       "requests " (:requests st) ", cache " (.size ^java.util.HashMap (:cache st)) " views, "
       (:bytes st) " chars, " (:hits st) " hits, " (:misses st) " misses\n"
       "heap " (let [r (Runtime/getRuntime)] (quot (- (.totalMemory r) (.freeMemory r)) 1048576)) " MB in use\n"
       "loaded " (ms (:started st)) " ms ago\n"))

(defn answer
  "One request on a state: [state' kind text], kind :ok, :error or :quit."
  [st line]
  (let [words (split-request line)
        st (update st :requests inc)]
    (case (first words)
      nil [st :error "empty request"]
      "quit" [st :quit ""]
      "ping" [st :ok ""]
      "stats" [st :ok (stats-text st)]
      "reload" (if (> (count words) 2)
                 [st :error "usage: reload [FILE.edn]"]
                 (let [file (or (second words) (:file st))
                       st' (try (open file) (catch Exception x x))]
                   (if (instance? Exception st')
                     [st :error (str "reload " file ": " (ex-message st'))]
                     (let [st' (assoc st' :requests (:requests st))]
                       [st' :ok (stats-text st')]))))
      (let [a (parse-args (conj words ""))]
        (if-not a
          [st :error usage-text]
          (let [k (dissoc a :time)
                ^java.util.HashMap cache (:cache st)]
            (if-let [text (.get cache k)]
              [(update st :hits inc) :ok text]
              (try
                (let [^String text (view-text (:ix st) a)
                      st (if (> (+ (long (:bytes st)) (.length text)) (long cache-limit))
                           (do (.clear cache) (assoc st :bytes 0))
                           st)]
                  (.put cache k text)
                  [(-> st (update :misses inc) (update :bytes + (.length text))) :ok text])
                (catch clojure.lang.ExceptionInfo x
                  [st :error (if (:view (ex-data x)) (ex-message x) usage-text)])))))))))

(defn show
  "Prints the answer to a request line, at a REPL: (show st \"uses p_wiv --ids\")."
  [st line]
  (print (nth (answer st line) 2))
  (flush))

(defn- respond [^java.io.OutputStream out kind ^String text]
  (let [^String body (if (and (= kind :error) (not (str/ends-with? text "\n"))) (str text "\n") text)
        b (.getBytes body "UTF-8")]
    (.write out (.getBytes (str (name kind) " " (alength b) "\n") "UTF-8"))
    (.write out b)
    (.write out (.getBytes (if (or (zero? (alength b)) (str/ends-with? body "\n")) ";;end\n" "\n;;end\n") "UTF-8"))
    (.flush out)))

(defn serve
  "Answers requests on stdin until quit or the input's end."
  [file]
  (if-let [st (try (open file)
                   (catch Exception x (err "  view-clj     " file ": " (ex-message x)) nil))]
    (let [in (java.io.BufferedReader. (java.io.InputStreamReader. System/in "UTF-8"))
          out (java.io.BufferedOutputStream. System/out 65536)]
      (err "  view-clj     serving " file ": read " (:read-ms st) "ms, indexed " (:index-ms st) "ms")
      (loop [st st]
        (if-let [line (.readLine in)]
          (let [[st kind text] (try (answer st line)
                                    (catch Throwable x [st :error (str "internal: " x)]))]
            (if (= kind :quit)
              0
              (do (respond out kind text) (recur st))))
          0)))
    1))

(defn run [argv]
  (case (first argv)
    "--batch" (if (= 4 (count argv)) (apply batch (rest argv)) (do (err usage-text) 2))
    "--serve" (if (= 2 (count argv)) (serve (second argv)) (do (err usage-text) 2))
    (let [a (parse-args argv)]
      (if-not a
        (do (err usage-text) 2)
        (try
          (let [[ix loaded indexed] (load-index (peek (:pos a)))
                t0 (System/nanoTime)
                text (view-text ix a)
                built (ms t0)]
            (print text)
            (flush)
            (when (:time a)
              (err "  view-clj     read " loaded "ms, indexed " indexed "ms, the view built and printed " built "ms"))
            0)
          (catch clojure.lang.ExceptionInfo x
            (cond (:view (ex-data x)) (do (err "  view-clj     " (ex-message x)) 1)
                  :else (do (err usage-text) 2))))))))

(defn -main [& argv]
  ;; a thread with a deep stack: the forms nest, and the walks recurse
  (let [code (promise)
        t (Thread. nil
                   #(deliver code (try (run (vec argv))
                                       (catch Throwable x (.printStackTrace x) 1)))
                   "view" (* 256 1024 1024))]
    (.start t)
    (.join t)
    (shutdown-agents)
    (System/exit (long (deref code 0 1)))))
