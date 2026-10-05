;; gview.view: the views -- a relation written in steps, the tree built from
;; it, the named views and the printer -- crefactor/graph/view's view.go,
;; named.go and print.go in Clojure, printing what `go tool whim view`
;; prints, byte for byte (doc/GRAPH.md, *Views in Clojure, over the EDN*).
(ns gview.view
  (:require [clojure.string :as str]
            [gview.graph :as g :refer [id text kids lst? head is? args kid nkids]])
  (:import (java.util ArrayList HashSet LinkedHashMap)))

(set! *warn-on-reflection* true)

;; STEPS.
(defn parse-steps [spec]
  (let [ws (str/split (str/trim spec) #"\s+")
        out (mapv (fn [w]
                    (let [in (str/ends-with? w "<")
                          w (if in (subs w 0 (dec (count w))) w)]
                      (cond
                        (#{"refers" "typed" "contains"} w) {:kind w :in in}
                        (#{"inside" "^fn" "^stmt" "call"} w)
                        (if in
                          (throw (ex-info (str w "< has no direction") {:view true}))
                          {:kind ({"inside" "inside" "^fn" "holder" "^stmt" "statement" "call" "call"} w)})
                        :else (throw (ex-info (str "no step \"" w "\": refers, typed, contains (each with < for against the edge), inside, ^fn, ^stmt, call") {:view true})))))
                  (remove str/blank? ws))]
    (when (empty? out) (throw (ex-info "a spec of no steps" {:view true})))
    out))

(defn- by-id [ns] (sort-by id ns))

(defn- dedupe-ident [ns]
  (reduce (fn [out n] (if (identical? n (peek out)) out (conj out n))) [] ns))

(defn- hits
  "What a relation reached, grouped by node, each with its uses: [{:to :via}]
  in id order."
  [ats]
  (let [m (LinkedHashMap.)]
    (doseq [[n via] ats]
      (let [^ArrayList l (or (.get m n) (let [l (ArrayList.)] (.put m n l) l))]
        (when via (.add l via))))
    (->> (for [[n vs] m] {:to n :via (dedupe-ident (by-id vs))})
         (sort-by (comp id :to))
         vec)))

(defn steps-rel
  "The relation of a list of steps: a node's children, as hits."
  [steps]
  (fn [ix n]
    (hits
     (reduce
      (fn [cur {:keys [kind in]}]
        (let [nxt (ArrayList.) seen (HashSet.)
              add (fn [n via] (when n (let [a [n via]] (when (.add seen a) (.add nxt a)))))]
          (doseq [[n via] cur]
            (case kind
              "refers" (if in
                         (doseq [d (g/decls ix n), u (g/uses ix d)] (add u u))
                         (doseq [r (g/refs n)] (add (g/rep ix (g/node ix r)) n)))
              "typed" (if in
                        (doseq [u (g/typed-by ix n)] (add u u))
                        (when-let [t (g/type-of ix n)] (add t n)))
              "contains" (if in
                           (add (g/parent ix n) via)
                           (doseq [k (kids n)] (when (not= 0 (id k)) (add k via))))
              "inside" (g/walk n (fn [k] (when (and (not (identical? k n)) (not= 0 (id k))) (add k via)) true))
              "holder" (add (g/rep ix (g/holder ix n)) via)
              "statement" (add (g/statement ix n) via)
              "call" (when (g/in-call? ix n) (add n via))))
          (vec nxt)))
      [[n nil]] steps))))

;; CONTEXTS AND THE TREE.
(defn use-form "A use with the least around it that says what it is." [ix u]
  (let [p (g/parent ix u)]
    (cond
      (nil? p) u
      (g/in-call? ix u) p
      (and (or (is? p "->") (is? p ".")) (> (nkids p) 2) (not (identical? (kid p 1) u))) p
      (is? p "at") p
      :else u)))

(defn contexts [ix uses show label]
  (when (and (not= show :none) (seq uses))
    (let [m (LinkedHashMap.)]
      (doseq [u uses]
        (let [f (case show :node (use-form ix u) :fn (g/holder ix u) (g/statement ix u))
              ^ArrayList l (or (.get m f) (let [l (ArrayList.)] (.put m f l) l))]
          (.add l u)))
      (->> (for [[f us] m]
             (let [us (vec us)]
               (cond-> {:form f :uses us :whole (boolean (#{:fn :node} show))}
                 label (assoc :label (label ix us)))))
           (sort-by (comp id :form))
           vec))))

(defn build
  "The view of root by spec: the root, its children, theirs, to the spec's
  depth, a revisit a link; children in id order, depth first."
  [ix root {:keys [name child rel depth stop show label]}]
  (let [root (g/rep ix root)
        held (HashSet. ^java.util.Collection (list root))
        stop (set stop)]
    (letfn [(expand [n d]
              (mapv (fn [{:keys [to via]}]
                      (let [c {:head child :node to :contexts (contexts ix via show label)}]
                        (if (.contains held to)
                          (assoc c :link true)
                          (do (.add held to)
                              (assoc c :kids (if (and (or (zero? depth) (< d depth)) (not (stop (head to))))
                                               (expand to (inc d))
                                               []))))))
                    (rel ix n)))]
      {:head name :node root :kids (expand root 1)})))

;; THE NAMED VIEWS.
(defn- plural [n word] (if (= n 1) (str "1 " word) (str n " " word "s")))

(defn- opt-depth [o d]
  (let [x (:depth o 0)] (cond (pos? x) x (neg? x) 0 :else d)))

(def ^:private accesses ["call" "read" "write" "update" "addr" "init" "type" "unevaluated" "label"])
(defn- access-order [a] (let [i (.indexOf ^java.util.List accesses a)] (if (neg? i) (count accesses) i)))

(defn- type-of* [ix e] (or (g/type-of ix e) (some->> (g/ref1 ix e) (g/type-of ix))))

(def ^:private compound #{"+=" "-=" "*=" "/=" "%=" "<<=" ">>=" "&=" "^=" "|="})
(def ^:private unevaluated #{"sizeof" "sizeof-bare" "alignof" "alignof-bare" "typeof" "typeof_unqual" "__typeof__" "__typeof"})

(defn access "What a use does with what it names, read off the forms around it." [ix u]
  (let [r (g/ref1 ix u)]
    (cond
      (and r (or (is? r "typedef") (is? r "extern-typedef") (g/type-def? r)
                 (str/starts-with? (head r) "extern-struct") (is? r "extern-union") (is? r "extern-enum")))
      "type"
      (and r (is? r "label")) "label"
      (g/in-call? ix u) "call"
      :else
      (let [p (g/parent ix u)]
        (cond
          (nil? p) "read"
          (is? p "at") "init"
          (and (is? p "->") (not (identical? (peek (kids p)) u))) "read"
          :else
          (loop [e (if (or (is? p "->") (is? p ".")) p u)]
            (let [q (g/parent ix e)]
              (if (nil? q)
                "read"
                (let [first? (and (> (nkids q) 1) (identical? (kid q 1) e))
                      h (head q)]
                  (cond
                    (= h "paren") (recur q)
                    (and (= h ".") first?) (recur q)
                    (and (= h "index") first? (some-> (type-of* ix e) (is? "array"))) (recur q)
                    (and (= h "=") first?) "write"
                    (or (and first? (compound h)) (#{"post++" "post--" "pre++" "pre--"} h)) "update"
                    (= h "addr") "addr"
                    (unevaluated h) "unevaluated"
                    :else "read"))))))))))

(defn- access-label [ix uses]
  (str/join "+" (sort-by access-order (distinct (map #(access ix %) uses)))))

(defn- tally [ix n holders]
  (let [us (for [d (g/decls ix n), u (g/uses ix d)] (access ix u))
        counts (frequencies us)]
    (str (plural (count us) "use") " of " (g/name-of ix n) " in " (plural holders "top-level form") ": "
         (if (empty? counts)
           "none"
           (str/join ", " (for [k (sort-by access-order (keys counts))] (str (counts k) " " k)))))))

(def callers-steps "refers< call ^fn")
(def callees-steps "inside call refers")
(def uses-steps "refers< ^fn")

(defn callers [ix f o]
  (let [t (build ix f {:name "callers" :child "in" :rel (steps-rel (parse-steps callers-steps))
                       :depth (opt-depth o 2) :stop (:stop o) :show (:show o :stmt)})
        n (:node t)
        us (for [d (g/decls ix n), u (g/uses ix d)] (g/in-call? ix u))
        calls (count (filter true? us)) other (- (count us) calls)]
    (assoc t :notes (cond-> [(str (plural calls "call") " of " (g/name-of ix n) " in " (plural (count (:kids t)) "function"))]
                      (pos? other) (conj (str "and " (plural other "use") " not in a call, which `uses " (g/name-of ix n) "` shows"))))))

(defn callees [ix f o]
  (let [t (build ix f {:name "callees" :child "calls" :rel (steps-rel (parse-steps callees-steps))
                       :depth (opt-depth o 2) :stop (:stop o) :show (:show o :stmt)})]
    (assoc t :notes [(str (g/name-of ix (:node t)) " calls " (plural (count (:kids t)) "function"))])))

(defn uses [ix n o]
  (let [t (build ix n {:name "uses" :child "in" :rel (steps-rel (parse-steps uses-steps))
                       :depth (opt-depth o 1) :stop (:stop o) :show (:show o :stmt) :label access-label})]
    (assoc t :notes [(tally ix (:node t) (count (:kids t)))])))

(defn member [ix m o] (assoc (uses ix m o) :head "member"))

(defn follow [ix root spec o]
  (let [d (:depth o 0)]
    (assoc (build ix root {:name "follow" :child "to" :rel (steps-rel (parse-steps spec))
                           :depth (cond (zero? d) 1 (neg? d) 0 :else d) :stop (:stop o) :show (:show o :stmt)})
           :notes [(str "steps: " spec)])))

(defn def-of [ix n] (g/rep ix n))

;; THE TYPE VIEW.
(defn- operand-of [ix t] (when (pos? (nkids t)) (g/type-of ix (peek (kids t)))))

(defn- reaches? [ix t s]
  (loop [t t i 0]
    (cond (or (nil? t) (>= i 64)) false
          (identical? t s) true
          (not (or (is? t "pointer") (is? t "array"))) false
          :else (recur (operand-of ix t) (inc i)))))

(defn- uses-attr [counts]
  (str "(uses " (reduce + 0 (vals counts))
       (apply str (for [k (sort-by access-order (keys counts))] (str " (" k " " (counts k) ")")))
       ")"))

(defn- member-name [m]
  (cond (and (is? m "member") (> (nkids m) 1)) (text (kid m 1))
        (and (pos? (nkids m)) (not (lst? (kid m 0)))) (text (kid m 0))
        :else ""))

(defn type-view [ix s _o]
  (let [label (if (or (is? s "enum") (is? s "extern-enum")) "enumerators" "members")
        flat (fn flat [s]
               (mapcat (fn [m]
                         (cond
                           (or (is? m "static_assert") (not (lst? m))) []
                           (and (pos? (nkids m)) (lst? (kid m 0)) (g/type-def? (kid m 0))) (flat (kid m 0))
                           :else
                           [(cond-> {:head (if (is? s "enum") "enumerator" "member") :node m :name (member-name m)
                                     :attrs (uses-attr (frequencies (map #(access ix %) (g/uses ix m))))}
                              (and (not (is? m "member")) (> (nkids m) 1) (not (is? s "enum")))
                              (assoc :inline [(kid m 1)]))]))
                       (if (g/type-def? s) (g/members s) (args s))))
        ms (cond-> {:head label :kids (vec (flat s))}
             (or (is? s "extern-struct") (is? s "extern-union")) (assoc :notes ["a header's: the members the file names"]))
        forms (:forms (:g ix))
        seen (HashSet.)
        taking (ArrayList.) returning (ArrayList.)]
    (doseq [f forms]
      (let [ft (g/decl-type f)]
        (when (and ft (is? ft "fn") (not (is? f "typedef")))
          (let [r (g/rep ix f)]
            (when (and (not (.contains seen r)) (identical? r f))
              (.add seen r)
              (let [ps (when (> (nkids ft) 1)
                         (filterv (fn [p] (when-let [t (g/type-of ix p)] (reaches? ix t s))) (kids (kid ft 1))))
                    ftt (g/type-of ix f)]
                (when (seq ps) (.add taking {:head "fn" :node f :inline ps}))
                (when (and ftt (is? ftt "function") (> (nkids ftt) 2) (reaches? ix (g/type-of ix (kid ftt 2)) s))
                  (.add returning {:head "fn" :node f :inline [(peek (kids ft))]}))))))))
    (let [holding (ArrayList.) objects (ArrayList.)]
      (doseq [f forms]
        (g/walk f (fn [n]
                    (if (is? n "defn")
                      false
                      (do (when (and (g/type-def? n) (not (identical? n s)) (or (is? n "struct") (is? n "union")))
                            (doseq [m (g/members n)]
                              (when (and (g/type-of ix m) (> (nkids m) 1) (not (lst? (kid m 0))) (reaches? ix (g/type-of ix m) s))
                                (.add holding {:head "member" :node m :inline [(kid m 1)]}))))
                          true))))
        (when (and (is? f "def") (g/type-of ix f) (reaches? ix (g/type-of ix f) s) (identical? (g/rep ix f) f))
          (.add objects {:head "def" :node f :inline [(g/decl-type f)]})))
      (let [groups [{:head "taking" :kids (vec taking)} {:head "returning" :kids (vec returning)}
                    {:head "held-in" :kids (vec holding)} {:head "objects" :kids (vec objects)}]]
        {:head "type" :node s
         :kids (into [ms] (filter (comp seq :kids)) groups)
         :notes [(str (g/name-of ix s) ": " (count (:kids ms)) " " label ", taken by " (plural (count taking) "function")
                      ", returned by " (count returning) ", held in " (plural (count holding) "member") ", "
                      (plural (count objects) "object"))]}))))

;; THE PRINTER.  A pdoc is a form ready to lay out: {:text} for an atom;
;; {:list true :pre :post :head :kids} for a list; :len its width on one
;; line (in bytes, as the Go counts) and :broken, computed once.
(def ^:private width 100)

(defn- blen ^long [^String s]
  (let [n (.length s)]
    (loop [i 0 b 0]
      (if (< i n)
        (let [c (int (.charAt s i))]
          (recur (inc i) (+ b (long (cond (< c 0x80) 1 (< c 0x800) 2 (Character/isSurrogate (char c)) 2 :else 3)))))
        b))))

(defn- broken? [{:keys [list head kids]}]
  (boolean
   (and list
        (case head
          "defn" true
          ("block" "stmt-expr") (or (> (count kids) 2) (and (= 2 (count kids)) (:broken (kids 1))))
          ("if" "while" "for" "do" "switch") (some :broken (rest kids))
          false))))

(defn- atom-doc [s] {:text s :len (blen s)})

(defn- list-doc [pre post hd ks]
  (let [d {:list true :pre pre :post post :head hd :kids ks
           :len (+ (blen pre) (blen post) 2 (max 0 (dec (count ks))) (reduce + 0 (map :len ks)))}]
    (assoc d :broken (broken? d))))

(def ^:private collapsible #{"block" "init" "struct" "union" "enum" "defn" "stmt-expr"})

(defn- refs-text [n] (apply str (map #(str "@" %) (g/refs n))))

(defn- doc [ids? n path]
  (if-not (lst? n)
    (atom-doc (if (or (not ids?) (zero? (id n))) (text n) (str "#" (id n) ":" (text n) (refs-text n))))
    (let [hd (head n)
          ks (loop [i 0 xs (kids n) out [] elided false]
               (if-let [k (first xs)]
                 (if (and path (pos? i) (lst? k) (not (.contains ^HashSet path k)) (or (collapsible hd) (is? k "block")))
                   (recur (inc i) (rest xs) (if elided out (conj out (atom-doc "..."))) true)
                   (recur (inc i) (rest xs) (conj out (doc ids? k path)) false))
                 out))]
      (list-doc (if (and ids? (not= 0 (id n))) (str "#" (id n)) "") (if ids? (refs-text n) "") hd ks))))

(defn- flat [^StringBuilder b d]
  (if-not (:list d)
    (.append b ^String (:text d))
    (do (.append b ^String (:pre d)) (.append b "(")
        (doseq [[i k] (map-indexed vector (:kids d))]
          (when (pos? i) (.append b " "))
          (flat b k))
        (.append b ")") (.append b ^String (:post d)))))

(defn- keep-n [{:keys [head kids]}]
  (let [atoms (count (take-while (complement :list) (rest kids)))]
    (case head
      ("if" "while" "switch" "return" "case" "sizeof" "cast" "call" "index" "." "->" "=" "literal" "static_assert" "?" "fn") 1
      "for" 3
      ("def" "typedef" "defn") (min (inc atoms) (dec (count kids)))
      atoms)))

(defn- spaces [n] (apply str (repeat n " ")))

(defn- layout [^StringBuilder b d col]
  (if (or (not (:list d)) (and (not (:broken d)) (<= (:len d) (- width col))))
    (flat b d)
    (let [ks (:kids d)]
      (.append b ^String (:pre d)) (.append b "(")
      (if (empty? ks)
        (do (.append b ")") (.append b ^String (:post d)))
        (let [fst (first ks)
              at0 (+ col (blen (:pre d)) 1)
              _ (layout b fst at0)
              [k at inner] (if (:list fst) [0 at0 (+ col 1)] [(keep-n d) (+ at0 (:len fst)) (+ col 2)])]
          (loop [i 0 xs (rest ks) k k at at]
            (when-let [x (first xs)]
              (if (and (< i k) (or (<= (:len x) (- width at 1)) (and (zero? i) (not (:list x)))))
                (do (.append b " ") (layout b x (inc at))
                    (recur (inc i) (rest xs) k (+ at 1 (:len x))))
                (do (.append b "\n") (.append b ^String (spaces inner)) (layout b x inner)
                    (recur (inc i) (rest xs) 0 at)))))
          (.append b ")") (.append b ^String (:post d)))))))

(defn- context-doc [ids? ix {:keys [form uses whole]}]
  (if whole
    (doc ids? form nil)
    (let [path (HashSet.)]
      (doseq [u uses]
        (loop [n u]
          (when (and n (.add path n) (not (identical? n form)))
            (recur (g/parent ix n)))))
      (doc ids? form path))))

(defn- entry-name [ids? ix t]
  (let [n (:node t)
        s (or (not-empty (:name t)) (g/name-of ix n))
        s (if (str/includes? s " ") (str "(" s ")") s)]
    (cond (and ids? (:link t)) (str "@" (id n) " " s)
          ids? (str "#" (id n) " " s)
          (:link t) (str "@" s)
          :else s)))

(defn- print-tree [^StringBuilder b ids? ix t col]
  (let [pad (spaces col)]
    (doseq [n (:notes t)] (.append b (str pad ";; " n "\n")))
    (.append b pad)
    (let [line (str "(" (:head t) (when (:node t) (str " " (entry-name ids? ix t))))]
      (.append b line)
      (loop [fs (:inline t) at (+ col (blen line))]
        (when-let [f (first fs)]
          (let [d (doc ids? f nil)]
            (if (<= (:len d) (- width at 1))
              (do (.append b " ") (layout b d (inc at)) (recur (rest fs) (+ at 1 (:len d))))
              (do (.append b (str "\n" pad "  ")) (layout b d (+ col 2)) (recur (rest fs) width))))))
      (when-let [a (:attrs t)] (.append b (str " " a))))
    (doseq [c (:contexts t)]
      (.append b (str "\n" (spaces (+ col 2))))
      (let [d (context-doc ids? ix c)
            d (if-let [l (not-empty (:label c))]
                (let [lt (str ":" l)] (list-doc "" "" lt [(atom-doc lt) d]))
                d)]
        (layout b d (+ col 2))))
    (doseq [k (:kids t)]
      (.append b "\n")
      (print-tree b ids? ix k (+ col 2)))
    (.append b ")")))

(defn tree-text "A view as Lisp, and a newline." [ids? ix t]
  (let [b (StringBuilder.)] (print-tree b ids? ix t 0) (.append b "\n") (str b)))

(defn form-text "One form, whole, from column 0, and a newline." [ids? n]
  (let [b (StringBuilder.)] (layout b (doc ids? n nil) 0) (.append b "\n") (str b)))
