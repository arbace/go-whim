;; gview.main: `go tool whim view`'s command line over a graph's EDN.
;;
;;   view-clj [--ids] [--depth N] [--show node|stmt|fn|none] [--stop HEADS] [--time]
;;            callers F | callees F | uses NAME | member S.M | type T | def NAME
;;            | follow 'STEPS' ROOT   FILE.edn
;;   view-clj --batch CASES OUTDIR FILE.edn
;;
;; FILE.edn is what `go tool whim graph --edn` writes.  The output is what
;; `go tool whim view` prints for the same arguments on the graph the EDN
;; was written from.  --batch reads the EDN once and runs every line of
;; CASES -- a view's arguments, tab-separated (or space-separated when the
;; line holds no tab) -- writing line N's output to OUTDIR/N.out and its
;; error to OUTDIR/N.err.  --c (def's C view) is the Go's alone: it wants
;; C's printer.
(ns gview.main
  (:require [clojure.java.io :as io]
            [clojure.string :as str]
            [gview.graph :as g]
            [gview.view :as v])
  (:gen-class))

(set! *warn-on-reflection* true)

(def ^:private usage-text
  "usage: view-clj [--ids] [--depth N] [--show node|stmt|fn|none] [--stop HEADS] [--time]\n                callers F | callees F | uses NAME | member S.M | type T | def NAME | follow 'STEPS' ROOT  FILE.edn\n       view-clj --batch CASES OUTDIR FILE.edn")

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

(defn run [argv]
  (if (= "--batch" (first argv))
    (if (= 4 (count argv)) (apply batch (rest argv)) (do (err usage-text) 2))
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
