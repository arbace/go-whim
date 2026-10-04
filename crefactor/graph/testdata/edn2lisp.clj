;; edn2lisp.clj: the graph's EDN read by Clojure's own EDN reader
;; (clojure.edn/read, the six tags given reader functions) and written back
;; as the graph's Lisp, one top-level form a line -- an independent reader
;; and writer of both notations, which crefactor/graph's TestCorpusEDNClojure
;; holds to the graph: the Lisp it writes, read by graph.Read, must be the
;; graph the EDN was written from, ids and edges (doc/GRAPH.md, *EDN*).
;;
;;   java -cp clojure.jar:spec.alpha.jar:core.specs.alpha.jar clojure.main \
;;     edn2lisp.clj IN.edn OUT.lisp [IN.edn OUT.lisp ...]

(require '[clojure.edn :as edn] '[clojure.java.io :as io])

(defrecord Node [id form edges])
(defrecord Ref [id])
(defrecord Typed [id])
(defrecord Tok [text])

(def readers
  {'g/n (fn [[id form & edges]] (->Node id form edges))
   'g/r ->Ref
   'g/t ->Typed
   'c/num ->Tok
   'c/char ->Tok
   'c/tok ->Tok})

(defn emit [^StringBuilder sb x]
  (cond
    (instance? Node x)
    (let [{:keys [id form edges]} x]
      (when (pos? id)
        (.append sb "#") (.append sb (str id))
        (when-not (seq? form) (.append sb ":")))
      (emit sb form)
      (doseq [e edges]
        (cond (instance? Ref e) (do (.append sb "@") (.append sb (str (:id e))))
              (instance? Typed e) (do (.append sb "@:") (.append sb (str (:id e))))
              :else (throw (ex-info "not an edge" {:edge e})))))
    (instance? Typed x) (do (.append sb "@:") (.append sb (str (:id x))))
    (instance? Tok x) (.append sb ^String (:text x))
    (seq? x) (do (.append sb "(")
                 (loop [xs x first? true]
                   (when (seq xs)
                     (when-not first? (.append sb " "))
                     (emit sb (first xs))
                     (recur (rest xs) false)))
                 (.append sb ")"))
    (string? x) (do (.append sb "\"") (.append sb ^String x) (.append sb "\""))
    (or (symbol? x) (boolean? x) (integer? x)) (.append sb (str x))
    :else (throw (ex-info "not a form" {:form x}))))

(defn convert [in out]
  (let [g (with-open [r (java.io.PushbackReader. (io/reader in))]
            (edn/read {:readers readers} r))
        sb (StringBuilder.)]
    (.append sb ";; the graph, from its EDN through clojure.edn\n")
    (doseq [f (:forms g)] (emit sb f) (.append sb "\n"))
    (doseq [[head k] [["types" :types] ["externs" :externs]]]
      (.append sb (str "(" head))
      (doseq [n (k g)] (.append sb "\n  ") (emit sb n))
      (.append sb ")\n"))
    (when-let [ids (:ids g)] (.append sb (str "(ids " ids ")\n")))
    (spit out (str sb))
    (count (:forms g))))

(doseq [[in out] (partition 2 *command-line-args*)]
  (println in (convert in out) "forms"))
