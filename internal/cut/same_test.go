package cut

import (
	"bytes"
	"os"
	"reflect"
	"regexp"
	"testing"

	"github.com/arbace/go-whim/crefactor/edit"
)

// sameTexts are the texts the regexps the cutters gave up are held to: the
// input and the product, and the boundaries the cutters actually read when
// a build has left them (phase 6 reads q003, phase 7 q006).
func sameTexts(t *testing.T) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	for _, f := range []string{"../../src/slim-vim.c", "../../src/whim-vim.c",
		"../../.cache/boundaries/q003.c", "../../.cache/boundaries/q006.c"} {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Logf("%s: %v -- skipped", f, err)
			continue
		}
		out[f] = src
	}
	return out
}

// TestCutterSearchesSame holds each search lfonly, noconv, onebuffer and
// keepbytes now make to the regexp it replaced, match for match.
func TestCutterSearchesSame(t *testing.T) {
	if testing.Short() {
		t.Skip("-short")
	}
	texts := sameTexts(t)
	texts["small"] = []byte("get_fileformat(x) xget_fileformat(y) _buf_hide( buf_hide(\n" +
		"mb_ptr2len mb_ptr2len_len (*mb_ptr2len)( bad_char bad_char_behavior int bad_char;\n" +
		"\tw_alt_fnum CMOD_HIDE CMOD_KEEPALTx buflist_altfpos(curwin) {Ctrl_HAT, nv_hat")
	for f, text := range texts {
		seen := 0
		// lfonly: `\bname\(` per dying function, every match's start used
		for _, n := range append(append([]string{}, lfonlyDying...), "buf_hide") {
			want := regexp.MustCompile(`\b`+n+`\(`).FindAllIndex(text, -1)
			if got := edit.CallsNotAfterWord(text, n); !reflect.DeepEqual(got, want) {
				t.Errorf("%s: %s(: %d calls, the regexp %d", f, n, len(got), len(want))
			}
			seen += len(want)
		}
		// noconv and keepbytes: `\bname\b` tested for any match at all
		blanked := bytes.ReplaceAll(text, []byte("int bad_char;"), nil)
		for _, p := range append(append([]struct{ ptr, fn string }{}, noconvPointers...),
			struct{ ptr, fn string }{"bad_char", ""}, struct{ ptr, fn string }{"mb_tail_off", ""}) {
			for _, s := range [][]byte{text, blanked} {
				want := regexp.MustCompile(`\b` + p.ptr + `\b`).Match(s)
				if got := edit.WordCount(s, p.ptr) > 0; got != want {
					t.Errorf("%s: %s: named %v, the regexp %v", f, p.ptr, got, want)
				}
			}
		}
		// onebuffer: its counted patterns, every match's start used
		for _, c := range oneBufferLeft {
			re := regexp.MustCompile(c.pattern)
			want := re.FindAllIndex(text, -1)
			if got := edit.AllIndex(re, text); !reflect.DeepEqual(got, want) {
				t.Errorf("%s: %s: %d matches, the regexp %d", f, c.pattern, len(got), len(want))
			}
			seen += len(want)
		}
		t.Logf("%s: %d matches compared", f, seen)
	}
}
