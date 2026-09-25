package p002

// Whim phase 2 -- the options for features that are not here.  See GOAL.md.
//
// Unlike phase 1, there are no commands to cut.  All fourteen `:menu` commands
// and all eight `:spell` ones are ALREADY `ex_ni` -- upstream's tiny
// configuration never compiled them, and the slim pipeline's empty-object prune
// removed their sources.  Checking that first is what stops this phase being
// busywork dressed as progress.
//
// What survived them is the SETTINGS.  Six spell options and one menu option are
// still in the table, still settable, still reported by `:set all` -- and read
// by nothing at all.  That is the same lie `:help` told in phase 1: a control
// the editor offers and cannot honour.  An embedded editor should say the option
// does not exist rather than accept a value and ignore it.
//
// `'mousemodel'` is NOT dropped, and the distinction is worth stating: it looks
// like a menu option and is not.  `:behave` sets it, and it selects how a mouse
// click behaves in a terminal, which this build still does.
//
// THE DELTA: `:set spell` and the six others become E518, and `:set all` stops
// listing them.  No Ex command changes -- they were already ex_ni -- so the
// cumulative list stays exactly what phase 1 left it, and the check below
// requires that rather than a new entry.
// --- first: prove there is nothing to cut ---------------------------------
// If a menu or spell command ever acquires a real handler again, this phase is
// no longer the whole story and should say so rather than quietly do half of it.
// --- cut the settings, and let the sweep find the rest --------------------

import (
	"io"
	"regexp"
	"strings"

	"github.com/arbace/go-whim/internal/phase"
)

var cmdRow = regexp.MustCompile(`\[CMD_(\w+)\] = \{\(char_u \*\)"([^"]*)", sizeof\([^)]*\) - 1,\s*(\w+)\s*,`)

// menuSpellCommands are every menu and spell command.  whim2 prints the ones
// that still have a real handler, and the shell REFUSES if the list is not
// empty: if a menu or spell command ever acquires one again, that phase is no
// longer the whole story and should say so rather than quietly do half of it.
var menuSpellCommands = strings.Fields(
	"menu amenu nmenu vmenu imenu cmenu omenu xmenu smenu tmenu unmenu " +
		"menutranslate emenu popup tunmenu tlmenu spell spellgood spellwrong " +
		"spellrare spellundo spelldump spellinfo spellrepall mkspell")

// Whim2Live prints every menu or spell command whose handler is not ex_ni.
func Whim2Live(text []byte, w io.Writer) error {
	rows := map[string]string{}
	for _, m := range cmdRow.FindAllSubmatch(text, -1) {
		rows[string(m[2])] = string(m[3])
	}
	var live []string
	for _, n := range menuSpellCommands {
		h, ok := rows[n]
		if !ok {
			h = "ex_ni"
		}
		if h != "ex_ni" {
			live = append(live, n)
		}
	}
	_, err := io.WriteString(w, strings.Join(live, " ")+"\n")
	return err
}

func init() { phase.RegisterQuery("whim2", Whim2Live) }
