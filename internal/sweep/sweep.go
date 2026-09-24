// Package sweep deletes what a cut left unreachable, to a fixpoint, all six
// kinds.
//
// The six feed each other -- deleting a function orphans a type, deleting a
// type orphans a prototype, deleting a field orphans an enumerator -- so none
// is finished until all are.  The canonicalisers were a seventh member of each
// round, to tidy what deletion leaves; they are not any more, because every
// phase ends with the canonical print (internal/build's finish), which does
// that and more, once, after the sweep.
package sweep

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/arbace/go-whim/internal/dead"
)

// MaxRounds is sweep.sh's ceiling.  Exceeding it is a hard failure.
const MaxRounds = 15

func digest(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// Sweep runs the loop over path and returns the number of rounds it took.
//
// Report lines go to w, one per round, in sweep.sh's format and order.
func Sweep(path string, w interface{ Write([]byte) (int, error) }) (rounds int, err error) {
	// The enumerator values of THIS input, kept for the whole sweep so every
	// round pins to the original numbering.  deadenums writes it the first
	// time something is dead and not before, so most sweeps never pay for the
	// -g build.  Like the shell's `mktemp -u`, the NAME is reserved and the
	// file is not created: deadenums asks whether it exists.
	valsFile, err := os.CreateTemp("", "enumvals.")
	if err != nil {
		return 0, err
	}
	vals := valsFile.Name()
	valsFile.Close()
	os.Remove(vals)
	defer os.Remove(vals)

	// A TOOL IS NOT RUN AGAIN ON TEXT IT HAS ALREADY PASSED.  Every one is a
	// pure function of the bytes, so a tool that ran and changed nothing on
	// text X cannot change it the next time the file is exactly X.  Clearing
	// the memory when a tool DOES change something is what keeps this from
	// changing the fixpoint.
	passed := map[string]string{}

	maxRounds := MaxRounds
	var tally *closureTally
	if closureOn {
		maxRounds = closureRounds
		tally = newTally()
	}

	for {
		rounds++
		src, err := os.ReadFile(path)
		if err != nil {
			return rounds, err
		}
		was := digest(src)

		typereach := runTypereach
		deadfields := runDeadfields
		deadenums := func(b []byte) ([]byte, string, error) { return runDeadenums(path, b, vals, dead.AnalyseEnums) }
		if closureOn {
			cr := &closureRound{path: path, tally: tally}
			typereach, deadfields = cr.typereach, cr.deadfields
			deadenums = func(b []byte) ([]byte, string, error) { return cr.deadenums(b, vals) }
		}

		said := make([]string, 6)
		order := []struct {
			name string
			run  func([]byte) ([]byte, string, error)
		}{
			{"deadsweep", func(b []byte) ([]byte, string, error) { return runDeadsweep(path, b) }},
			{"deadprotos", runDeadprotos},
			{"typereach", typereach},
			{"funcreach", runFuncreach},
			{"deadfields", deadfields},
			{"deadenums", deadenums},
		}

		cur := src
		for i, t := range order {
			now := digest(cur)
			if passed[t.name] == now {
				said[i] = fmt.Sprintf("  %-12s passed this text already", t.name)
				continue
			}
			out, line, err := t.run(cur)
			if err != nil {
				// sweep.sh's pass() runs each tool through a pipe to tail, so
				// a tool's failure is invisible to set -e and the round goes
				// on.  Reproduced: the text is left as it was and the tool
				// records nothing.
				said[i] = line
				passed[t.name] = ""
				continue
			}
			said[i] = line
			cur = out
			if digest(cur) == now {
				passed[t.name] = now
			} else {
				passed[t.name] = ""
			}
		}

		if !bytes.Equal(cur, src) {
			if err := os.WriteFile(path, cur, 0o644); err != nil {
				return rounds, err
			}
		}

		fmt.Fprintf(w, "  sweep %d      %s; %s; %s; %s; %s; %s\n",
			rounds, said[0], said[1], said[2], said[3], said[4], said[5])

		if digest(cur) == was {
			break
		}
		if rounds >= maxRounds {
			fmt.Fprintln(w, "  sweep        not converging")
			return rounds, fmt.Errorf("sweep: not converging after %d rounds", rounds)
		}
	}

	if tally != nil {
		fmt.Fprintln(w, tally.line())
	}

	// The values file exists only if some round actually dumped DWARF.  This
	// check's exit status is the ONE in the whole sweep that is not masked.
	if _, err := os.Stat(vals); err == nil {
		moved, gone, err := dead.VerifyEnums(path, vals)
		if err != nil {
			return rounds, err
		}
		if len(moved) > 0 {
			n := len(moved)
			if n > 8 {
				n = 8
			}
			fmt.Fprintf(w, "  enumvals     %d surviving enumerators changed value: %v\n",
				len(moved), moved[:n])
			return rounds, fmt.Errorf("sweep: %d surviving enumerators changed value", len(moved))
		}
		fmt.Fprintf(w, "  enumvals     %d enumerators gone, and not one survivor moved\n", gone)
	}

	return rounds, nil
}
