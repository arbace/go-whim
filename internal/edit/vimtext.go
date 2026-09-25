package edit

import (
	"github.com/arbace/go-whim/crefactor/text"
	"github.com/arbace/go-whim/internal/whim/vimtext"
)

// What the phases share that knows vim moved to internal/whim/vimtext
// (doc/VIM-VS-GENERIC.md section 4, migration step 4): the buffer walks,
// Key, the command table's residue check and the W80, W88, W119, W126 and
// W127 helpers.  Every name is forwarded here, so the phases read as they
// did.

const (
	BwdWalk = vimtext.BwdWalk
	FwdWalk = vimtext.FwdWalk
)

var (
	W119DeclRe   = vimtext.W119DeclRe
	W119Dir      = vimtext.W119Dir
	W119Field    = vimtext.W119Field
	W119Inc      = vimtext.W119Inc
	W119Name     = vimtext.W119Name
	W119ProtoGP  = vimtext.W119ProtoGP
	W119Reraise  = vimtext.W119Reraise
	W119RetType  = vimtext.W119RetType
	W119Write    = vimtext.W119Write
	W127Decl     = vimtext.W127Decl
	W127DlText   = vimtext.W127DlText
	W127FnHead   = vimtext.W127FnHead
	W127Interior = vimtext.W127Interior
	W127MlFlags  = vimtext.W127MlFlags
	W127NotDecl  = vimtext.W127NotDecl
	W80Banner    = vimtext.W80Banner
	W80Case      = vimtext.W80Case
	W80Chars     = vimtext.W80Chars
	W80Count     = vimtext.W80Count
	W80EnumRe    = vimtext.W80EnumRe
	W80Fall      = vimtext.W80Fall
	W80IdRe      = vimtext.W80IdRe
	W80Idx1      = vimtext.W80Idx1
	W80Idx2      = vimtext.W80Idx2
	W80Label     = vimtext.W80Label
	W80Num       = vimtext.W80Num
	W80RowRe     = vimtext.W80RowRe
	W80Skip      = vimtext.W80Skip
	W80Table     = vimtext.W80Table
	W80Vim9      = vimtext.W80Vim9
	W80Word      = vimtext.W80Word
)

func CoreResidue(p text.Ph, t []byte, dying []string) (int, []string, []string, error) {
	return vimtext.CoreResidue(p, t, dying)
}

func CoreRows(t []byte) [][]byte { return vimtext.CoreRows(t) }

func Key(a, b string) string { return vimtext.Key(a, b) }

func W119IsDecl(l string) bool { return vimtext.W119IsDecl(l) }

func W119Or(xs []string) string { return vimtext.W119Or(xs) }

func W126PyList(s []string) string { return vimtext.W126PyList(s) }

func W126PyRepr(s string) string { return vimtext.W126PyRepr(s) }

func W127Index(lines []string, s string) int { return vimtext.W127Index(lines, s) }

func W127Splice(lines []string, a, b int, rows []string) []string {
	return vimtext.W127Splice(lines, a, b, rows)
}

func W88Lines(s string) []string { return vimtext.W88Lines(s) }
