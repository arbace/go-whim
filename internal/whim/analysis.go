package whim

import (
	"bytes"
	"regexp"

	"github.com/arbace/go-whim/crefactor/ccx"
	"github.com/arbace/go-whim/crefactor/reach"
)

// What the analysis tools are told about vim (doc/VIM-VS-GENERIC.md, step 7):
// internal/dead's funcreach, internal/reach's closure and internal/ccx's
// checks.  Each of them hard-coded these names before; none of them names
// anything in vim now.

// allocators are vim's allocators, whose void * result is fresh memory; the
// core's host_alloc is one more where the analysis reads the whole core.
var allocators = []string{"alloc", "alloc_clear", "lalloc", "lalloc_clear"}

// byteFuncs are the functions of bytes the core calls: musl's, since phase 98.
var byteFuncs = []string{"musl_memmove", "musl_memcpy", "musl_memset", "musl_memcmp"}

// Dead is what crefactor/dead's funcreach is told: vim's one entry point,
// and the floor its answer is checked against -- funcreach.py's: fewer
// definitions than this in vim's file means the shape funcreach matches has
// changed, and acting on the answer would delete most of the program.  Both
// were hard-coded in internal/dead/funcreach.go.
var Dead = struct {
	Roots          []string
	MinDefinitions int
}{Roots: []string{"main"}, MinDefinitions: 100}

// Reach is what internal/reach's closure is told: ml_recover, whose
// definition means the editor still reads swap files, so a struct layout is a
// disk format; and the allocators, a cast of whose result is no pun.  They
// were hard-coded in internal/reach/reach.go and cast.go.
var Reach = reach.Options{
	FreezeLayoutIf: []string{"ml_recover"},
	Allocators:     append(append([]string{}, allocators...), "host_alloc"),
	Core:           reachCore,
}

// reachCore is where whim-vim.c's core ends for the closure: the start of
// the line of the first #include, the host's first line (CLAUDE.md, The core
// and the host).
func reachCore(src []byte) int {
	if i := bytes.Index(src, []byte("\n#include")); i >= 0 {
		return i + 1
	}
	if bytes.HasPrefix(src, []byte("#include")) {
		return 0
	}
	return -1
}

// q is a literal in a regexp.
func q(s string) string { return regexp.QuoteMeta(s) }

// CCX is what internal/ccx's checks are told about the core (internal/gen/pre
// runs them on editor.c).  It was hard-coded in internal/ccx: casts.go,
// voids.go, order.go, garrays.go and unions.go.
var CCX = ccx.Profile{
	Allocators: append(append([]string{}, allocators...), "host_alloc"),
	Frees:      []string{"host_free"},
	ByteFuncs:  byteFuncs,
	// musl's string and character functions, and gettext and _, the
	// identity once gettext went.
	PureCalls: []string{
		"musl_strlen", "musl_strcmp", "musl_strncmp", "musl_strcasecmp", "musl_strncasecmp",
		"musl_strchr", "musl_strrchr", "musl_strstr", "musl_strpbrk", "musl_memcmp",
		"musl_isdigit", "musl_isalpha", "musl_isspace", "musl_isupper", "musl_islower",
		"musl_isalnum", "musl_isxdigit", "musl_toupper", "musl_tolower", "musl_isprint",
		"musl_iscntrl", "musl_ispunct", "musl_isgraph", "musl_atoi", "musl_strtol",
		"gettext", "_",
	},
	// vim's growarray: garray_T, its storage ga_data and its element size
	// ga_itemsize, which ga_init2's second argument sets and ga_grow keeps.
	GrowArray: ccx.GrowArray{
		Type: "garray_T", Tag: "growarray", Data: "ga_data", ItemSize: "ga_itemsize",
		Init: "ga_init2", InitSize: 1, Grow: "ga_grow",
	},
	Unions: ccx.UnionProfile{
		// The discriminants.  attrentry_T.ae_u holds term in term_attr_table
		// and cterm in cterm_attr_table; which table a highlight reads is
		// t_colors > 1.  The regexp's saved positions are pos for a
		// multi-line match (rex.reg_match == NULL) and ptr for a string.  A
		// regstack item holds sesave in the states that save a
		// subexpression's start or end, regsave in the others.
		Rules: []ccx.UnionRule{
			{Union: "ae_u", Member: map[string]*regexp.Regexp{
				"term":  regexp.MustCompile(`^(-` + q("t_colors>1") + `|\+` + q("table==&term_attr_table") + `|before:.*` + q("get_attr_entry(&term_attr_table,&") + `\{base\}\))$`),
				"cterm": regexp.MustCompile(`^(\+` + q("t_colors>1") + `|\+` + q("table==&cterm_attr_table") + `|before:.*` + q("get_attr_entry(&cterm_attr_table,&") + `\{base\}\))$`),
			}},
			{Union: "rs_u", Member: map[string]*regexp.Regexp{
				"pos": regexp.MustCompile(`^\+` + q("rex.reg_match==nullptr") + `$`),
				"ptr": regexp.MustCompile(`^-` + q("rex.reg_match==nullptr") + `$`),
			}},
			{Union: "se_u", Member: map[string]*regexp.Regexp{
				"pos": regexp.MustCompile(`^\+` + q("rex.reg_match==nullptr") + `$`),
				"ptr": regexp.MustCompile(`^-` + q("rex.reg_match==nullptr") + `$`),
			}},
			{Union: "rs_un", Member: map[string]*regexp.Regexp{
				"sesave":  regexp.MustCompile(`^(case:(RS_MOPEN|RS_MCLOSE)|after:.*regstack_push\((RS_MOPEN|RS_MCLOSE),.*)$`),
				"regsave": regexp.MustCompile(`^(case:(RS_BRANCH|RS_BRCPLX_MORE|RS_BRCPLX_LONG|RS_BRCPLX_SHORT|RS_NOMATCH|RS_BEHIND1|RS_BEHIND2|RS_STAR_LONG|RS_STAR_SHORT)|after:.*regstack_push\((RS_BRANCH|RS_BRCPLX_MORE|RS_BRCPLX_LONG|RS_BRCPLX_SHORT|RS_NOMATCH|RS_BEHIND1|RS_STAR_LONG|RS_STAR_SHORT|rst\.minval<=rst\.maxval\?RS_STAR_LONG:RS_STAR_SHORT),.*)$`),
			}},
		},
		// An option's old and new value hold the member of the option's
		// kind: the options[] table's P_BOOL is boolean, P_NUM number and
		// P_STRING string; the setters write the one member their kind
		// holds, then call the row's did_set_ callback.
		Kinded: &ccx.KindedUnion{
			Unions:   []string{"os_oldval", "os_newval"},
			Table:    "options",
			Kinds:    map[string]string{"P_BOOL": "boolean", "P_NUM": "number", "P_STRING": "string"},
			Callback: regexp.MustCompile(`\b(did_set_\w+)\b`),
			Setters:  map[string]string{"set_bool_option": "boolean", "set_num_option": "number", "did_set_string_option": "string"},
			Flags:    "flags",
			Written:  "an option's old and new value, written by the setter of its kind",
			Read:     "an option's old and new value, read by a callback only rows of its kind name",
		},
		// Which table a highlight reads is t_colors > 1.
		Guarded: ccx.GuardedCalls{
			Funcs: []ccx.GuardedCall{
				{Func: "syn_cterm_attr2entry", Guard: "+t_colors>1"},
				{Func: "syn_term_attr2entry", Guard: "-t_colors>1"},
			},
			Var:   "t_colors",
			Class: "a highlight table read where t_colors names it",
		},
		// A regstack item's state moves only within the class of its member.
		State: &ccx.StateField{
			Member: "rs_state", Push: "regstack_push", Param: "state",
			Class:  regexp.MustCompile(`^(RS_MOPEN|RS_MCLOSE)$`),
			Pushed: "a regstack item's state, set by the push that names it",
			Moved:  "a regstack item's state, moved within its member's class",
		},
		Discriminants: []ccx.Discriminant{
			{Var: "t_colors", Guard: regexp.MustCompile(`^[+-]t_colors>1$`)},
			{Var: "rex.reg_match", Guard: regexp.MustCompile(`^[+-]rex\.reg_match==nullptr$`)},
		},
		// A function that saves rex on entry and restores it before it
		// returns leaves rex.reg_match as it found it.
		Saves: &ccx.SavedVar{Var: "rex", Copy: "rex_save", For: "rex.reg_match"},
	},
}
