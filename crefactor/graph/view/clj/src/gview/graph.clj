;; gview.graph: the graph read from its EDN by Clojure's own reader
;; (clojure.edn, the six tags given reader functions), the forms' syntax a
;; view asks of a declaration, and the index -- crefactor/graph's Node,
;; names.go and forms.go, and crefactor/graph/view's index.go, in Clojure
;; (doc/GRAPH.md, *Views in Clojure, over the EDN*).  Nothing here names a
;; program: a root is found by its name in the file, a member, or an id.
(ns gview.graph
  (:require [clojure.edn :as edn]
            [clojure.java.io :as io]
            [clojure.string :as str])
  (:import (java.util ArrayList HashSet)))

(set! *warn-on-reflection* true)

;; A node: its id (0 for a token), its text (an atom's; "" for a list), its
;; elements (a vector for a list, nil for an atom), its refers edges (ids)
;; and its typed edge (an id, 0 for none).  The edges are ids, resolved
;; through the index; a node's identity is the object's, as a pointer is in
;; the Go -- deftype keeps Object's equals and hashCode.
(deftype Node [^long id ^String text kids refs ^long type])

(deftype Ref [^long id])

;; THE EDN.  #g/t alone is a type's operand: an atom of no text, typed.
(defn- tok [^String s] (Node. 0 s nil nil 0))

(defn- ->node [x]
  (cond
    (instance? Node x) x
    (seq? x) (Node. 0 "" (mapv ->node x) nil 0)
    (string? x) (tok (str "\"" x "\""))
    (or (symbol? x) (boolean? x) (integer? x)) (tok (str x))
    :else (throw (ex-info "not a form" {:form x}))))

(defn- read-node [v]
  (let [id (long (nth v 0))
        form (nth v 1)
        n (if (seq? form)
            (Node. id "" (mapv ->node form) nil 0)
            (Node. id (.-text ^Node (->node form)) nil nil 0))
        edges (subvec v 2)
        refs (not-empty (into [] (comp (filter #(instance? Ref %)) (map #(.-id ^Ref %))) edges))
        typ (reduce (fn [t e] (if (instance? Node e) (.-type ^Node e) t)) 0 edges)]
    (Node. id (.-text n) (.-kids n) refs typ)))

(def readers
  {'g/n read-node
   'g/r (fn [id] (Ref. id))
   'g/t (fn [id] (Node. 0 "" nil nil (long id)))
   'c/num tok
   'c/char tok
   'c/tok tok})

(defn read-edn
  "The graph of an EDN file: {:forms :types :externs :ids}, every section
  a vector of nodes."
  [file]
  (let [g (with-open [r (java.io.PushbackReader. (io/reader file) 65536)]
            (edn/read {:readers readers} r))]
    (-> g
        (update :forms #(mapv ->node %))
        (update :types #(mapv ->node %))
        (update :externs #(mapv ->node %)))))

;; THE NODE.
(defn id ^long [^Node n] (.-id n))
(defn text ^String [^Node n] (.-text n))
(defn kids [^Node n] (or (.-kids n) []))
(defn lst? [^Node n] (and (some? n) (some? (.-kids n))))
(defn refs [^Node n] (.-refs n))
(defn nkids ^long [^Node n] (count (.-kids n)))
(defn kid ^Node [^Node n i] (nth (.-kids n) i))

(defn head ^String [^Node n]
  (if (and (lst? n) (pos? (nkids n)) (not (lst? (kid n 0))))
    (text (kid n 0))
    ""))

(defn is? [n h] (= (head n) h))

(defn args [^Node n]
  (if (and (lst? n) (pos? (nkids n))) (subvec (.-kids n) 1) []))

(defn walk
  "Calls f on n and, while f says true, on what it contains."
  [^Node n f]
  (when (f n)
    (doseq [k (.-kids n)] (walk k f))))

;; THE FORMS' SYNTAX (forms.go, names.go).
(def ^:private prefix-words
  #{"static" "extern" "typedef" "register" "auto" "inline" "__inline" "__inline__"
    "_Noreturn" "_Thread_local" "thread_local" "__thread" "constexpr" "__auto_type"})

(defn- attr-form? [n] (or (is? n "attr") (is? n "attr-text") (is? n "std-attr")))

(defn- def-name-at ^long [n]
  (loop [i 1]
    (if (< i (nkids n))
      (let [k (kid n i)]
        (cond
          (or (and (not (lst? k)) (prefix-words (text k))) (attr-form? k)) (recur (inc i))
          (lst? k) 0
          :else i))
      0)))

(defn decl-name
  "The name a top-level def, typedef or defn declares, or \"\"."
  [f]
  (if (#{"def" "typedef" "defn"} (head f))
    (let [i (def-name-at f)] (if (pos? i) (text (kid f i)) ""))
    ""))

(defn decl-type
  "A def's, typedef's or defn's type form, or nil."
  [f]
  (let [i (def-name-at f)]
    (when (and (pos? i) (< (inc i) (nkids f))) (kid f (inc i)))))

(defn tag-of [n]
  (if (and (> (nkids n) 1) (not (lst? (kid n 1))) (not= "{}" (text (kid n 1))))
    (text (kid n 1))
    ""))

(defn- body [n]
  (let [as (args n)
        as (if (and (seq as) (not (lst? (first as))) (not= "{}" (text (first as)))) (subvec as 1) as)
        as (if (and (seq as) (is? (first as) "@")) (subvec as 1) as)]
    (if (and (is? n "enum") (seq as) (is? (first as) ":")) (subvec as 1) as)))

(defn type-def?
  "n is a struct, union or enum form that defines its type: it has a body."
  [n]
  (and (#{"struct" "union" "enum"} (head n)) (boolean (seq (body n)))))

(defn members [n]
  (filterv #(not (or (is? % "@") (and (not (lst? %)) (= "{}" (text %))))) (body n)))

;; THE INDEX: what the edges say the other way round.
(defrecord Index [g ^objects by-id ^objects parent ^objects uses ^objects typed-by top ^HashSet top-set])

(defn- add! [^objects a ^long i x]
  (let [^ArrayList l (or (aget a i) (let [l (ArrayList. 2)] (aset a i l) l))]
    (.add l x)))

(defn new-index [g]
  (let [secs [(:forms g) (:types g) (:externs g)]
        max-id (let [m (volatile! 0)]
                 (doseq [s secs, f s] (walk f (fn [n] (vswap! m max (id n)) true)))
                 @m)
        sz (inc max-id)
        by-id (object-array sz) parent (object-array sz)
        uses (object-array sz) typed-by (object-array sz)
        top-set (HashSet.)]
    (letfn [(iw [^Node n p]
              (let [i (.-id n)]
                (when (not= 0 i)
                  (aset by-id i n)
                  (aset parent i p))
                (doseq [r (.-refs n)]
                  (when (and (not= 0 (long r)) (not= 0 i)) (add! uses r n)))
                (let [t (.-type n)]
                  (when (and (not= 0 t) (not= 0 i)) (add! typed-by t n)))
                (doseq [k (.-kids n)] (iw k n))))]
      (doseq [s secs, f s]
        (.add top-set f)
        (iw f nil)))
    (->Index g by-id parent uses typed-by
             (reduce (fn [m f] (let [nm (decl-name f)]
                                 (if (= "" nm) m (update m nm (fnil conj []) f))))
                     {} (:forms g))
             top-set)))

(defn node
  "The node of an id, or nil."
  ^Node [^Index ix ^long i]
  (let [^objects a (.-by-id ix)]
    (when (and (pos? i) (< i (alength a))) (aget a i))))

(defn ref1 "n's first refers edge's node, or nil." [ix n]
  (when-let [r (first (refs n))] (node ix r)))

(defn type-of "n's typed edge's node, or nil." [ix ^Node n]
  (let [t (.-type n)] (when (not= 0 t) (node ix t))))

(defn parent [^Index ix n]
  (let [i (id n) ^objects a (.-parent ix)]
    (when (and (not= 0 i) (< i (alength a))) (aget a i))))

(defn- listed [^objects a n]
  (let [i (id n)]
    (if (and (not= 0 i) (< i (alength a))) (or (some-> ^ArrayList (aget a i) vec) []) [])))

(defn uses "Every node with a refers edge to n, in the walk's order." [^Index ix n] (listed (.-uses ix) n))
(defn typed-by [^Index ix n] (listed (.-typed-by ix) n))
(defn top? [^Index ix n] (.contains ^HashSet (.-top-set ix) n))

(defn holder "The top-level node holding n." [ix n]
  (loop [n n] (if-let [p (parent ix n)] (recur p) n)))

(defn statement "The statement holding n." [ix n]
  (loop [n n]
    (let [p (parent ix n)]
      (cond (nil? p) n
            (#{"block" "defn" "stmt-expr"} (head p)) n
            :else (recur p)))))

(defn decls "n's entity's declarations: every top-level form declaring its name." [^Index ix n]
  (or (when (top? ix n)
        (let [nm (decl-name n)]
          (when (not= "" nm) (not-empty ((.-top ix) nm)))))
      [n]))

(defn rep "The node that stands for n's entity: a function's definition, else its last declaration." [ix n]
  (let [ds (decls ix n)]
    (if (= 1 (count ds))
      (first ds)
      (or (first (filter #(is? % "defn") ds)) (peek ds)))))

(defn in-call? "n is what a call calls." [ix n]
  (let [p (parent ix n)]
    (and (some? p) (is? p "call") (> (nkids p) 1) (identical? (kid p 1) n))))

(defn params? "p is a function type's parameter list." [ix p]
  (let [g (parent ix p)]
    (and (some? g) (or (is? g "fn") (is? g "fn-ids")) (> (nkids g) 1) (identical? (kid g 1) p))))

(defn name-of "How a view names a node." [ix n]
  (cond
    (nil? n) ""
    (not (lst? n)) (text n)
    :else
    (let [dn (decl-name n) h (head n)]
      (cond
        (not= "" dn) dn
        (#{"struct" "union" "enum"} h)
        (let [t (tag-of n) p (parent ix n)]
          (cond (not= "" t) (str h " " t)
                (and p (is? p "typedef")) (decl-name p)
                (and p (parent ix p) (type-def? (parent ix p))) (name-of ix (parent ix p))
                :else h))
        (and (or (str/starts-with? h "extern") (= h "undeclared") (= h "member"))
             (> (nkids n) 1) (not (lst? (kid n 1))))
        (text (kid n 1))
        :else
        (or (when (and (pos? (nkids n)) (not (lst? (kid n 0))))
              (when-let [p (parent ix n)]
                (cond
                  (and (or (is? p "struct") (is? p "union")) (type-def? p))
                  (let [s (-> (name-of ix p) (str/replace-first #"^struct " "") (str/replace-first #"^union " ""))]
                    (str s "." (text (kid n 0))))
                  (or (is? p "enum") (params? ix p)) (text (kid n 0)))))
            h)))))

;; FINDING A ROOT.
(defn- first-in-forms [ix pred]
  (some (fn [f] (let [found (volatile! nil)]
                  (walk f (fn [n] (when (and (nil? @found) (pred n)) (vreset! found n)) (nil? @found)))
                  @found))
        (:forms (:g ix))))

(defn tag-def [ix kw tag]
  (first-in-forms ix #(and (is? % kw) (= (tag-of %) tag) (type-def? %))))

(defn aggregate-of [ix n]
  (cond
    (or (type-def? n) (#{"extern-struct" "extern-union" "extern-enum"} (head n))) n
    (let [t (type-of ix n)] (and t (not (identical? n t)))) (aggregate-of ix (type-of ix n))))

(defn- find-member [s m]
  (some (fn [x]
          (cond
            (and (is? x "member") (> (nkids x) 1) (= m (text (kid x 1)))) x
            (and (pos? (nkids x)) (not (lst? (kid x 0))) (= m (text (kid x 0)))) x
            (and (pos? (nkids x)) (lst? (kid x 0)) (type-def? (kid x 0))) (find-member (kid x 0) m)))
        (if (type-def? s) (members s) (args s))))

(defn- fail [msg] (throw (ex-info msg {:view true})))

(declare find-roots)

(defn aggregate "The struct, union or enum a name says." [^Index ix nm]
  (let [nm (str/trim nm)]
    (or (if (or (str/includes? nm " ") (str/starts-with? nm "#"))
          (or (aggregate-of ix (first (find-roots ix nm)))
              (fail (str (head (first (find-roots ix nm))) " is not a struct, union or enum")))
          (or (some #(when (is? % "typedef") (aggregate-of ix %)) ((.-top ix) nm))
              (some #(tag-def ix % nm) ["struct" "union" "enum"])
              (some #(when (and (str/starts-with? (head %) "extern-") (> (nkids %) 1) (= nm (text (kid % 1))))
                       (aggregate-of ix %))
                    (:externs (:g ix)))))
        (fail (str "no struct, union or enum is named " nm)))))

(defn- locals [^Index ix fname x]
  (let [f (last (filter #(is? % "defn") ((.-top ix) fname)))]
    (when-not f (fail (str "no function " fname " is defined")))
    (let [out (ArrayList.)]
      (walk f (fn [n]
                (when (and (not (identical? n f)) (lst? n) (not= 0 (id n)))
                  (if (and (is? n "def") (= x (decl-name n)))
                    (.add out n)
                    (let [p (parent ix n)]
                      (when (and p (params? ix p) (> (nkids n) 1) (= x (text (kid n 0))))
                        (.add out n)))))
                true))
      (when (.isEmpty out) (fail (str fname " declares no " x)))
      (vec out))))

(defn find-roots
  "The nodes a view is rooted at, by what a user would type: #ID, NAME,
  S.M, struct T, F/x."
  [^Index ix what]
  (let [what (str/trim what)]
    (cond
      (str/starts-with? what "#")
      (let [n (try (let [v (Long/parseLong (subs what 1))]
                     (when (<= 0 v 0xffffffff) (node ix v)))
                   (catch NumberFormatException _ nil))]
        (if n [n] (fail (str "no node " what))))
      (str/includes? what "/")
      (let [i (str/index-of what "/")] (locals ix (subs what 0 i) (subs what (inc i))))
      (str/includes? what ".")
      (let [i (str/index-of what ".") s (subs what 0 i) m (subs what (inc i))
            t (aggregate ix s)]
        (if-let [mem (find-member t m)] [mem] (fail (str s " has no member " m))))
      (some #(str/starts-with? what %) ["struct " "union " "enum "])
      (let [i (str/index-of what " ")]
        (if-let [t (tag-def ix (subs what 0 i) (str/trim (subs what (inc i))))]
          [t] (fail (str "no " what " defined"))))
      :else
      (if-let [ds (not-empty ((.-top ix) what))]
        [(rep ix (first ds))]
        (or (some (fn [f]
                    (let [found (volatile! nil)]
                      (walk f (fn [n]
                                (when (and (nil? @found) (is? n "enum"))
                                  (vreset! found (first (filter #(and (lst? %) (pos? (nkids %)) (= what (text (kid % 0))) (not= 0 (id %)))
                                                                (args n)))))
                                (nil? @found)))
                      (some-> @found vector)))
                  (:forms (:g ix)))
            (some #(when (and (> (nkids %) 1) (= what (text (kid % 1)))) [%]) (:externs (:g ix)))
            (some-> (tag-def ix "struct" what) vector)
            (some-> (tag-def ix "union" what) vector)
            (fail (str "nothing in the file declares " what)))))))
