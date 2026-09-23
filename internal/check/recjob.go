package check

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// A RECORDING THAT FAILS MUST SAY WHY, and this is how every check here runs
// one.
//
// The checks started recorders in the background and collected them with a
// bare wait, so a stalled recording -- tools/zpty.py reaching its deadline under
// load, a zmemline case that never returned -- ended the phase with rc 1 and
// NOTHING printed; and where a status was discarded instead, a recording that
// died left a short directory and the comparison after it reported a record
// as MOVED.  The first costs a reader the reason, and the reason is what tells
// a busy machine from a wrong phase.  The second is worse: it is a verdict
// that is true of both outcomes.  So a recorder's output is KEPT rather than
// thrown away, and a failure is reported with which recording it was, its
// exit status and the last lines it wrote.

type RecJob struct {
	What string // how the report names it: "the recording of the input"
	argv []string
	how  string                // what the report calls the command, when there is no argv
	fn   func(io.Writer) error // a recorder in this process rather than a child
	Out  bytes.Buffer
	Err  error
}

// NewRec is one recorder, not yet run, as a child: `sh tools/st.sh
// zcases|zmemline|ztermcheck BIN DIR`.
func NewRec(what string, argv ...string) *RecJob { return &RecJob{What: what, argv: argv} }

// NewRecFunc is one recorder, not yet run, IN THIS PROCESS -- a whole
// recording is harness.ZRecord and not a shell that starts six children -- with
// how naming it the way an argv would.
func NewRecFunc(what, how string, fn func(io.Writer) error) *RecJob {
	return &RecJob{What: what, how: how, fn: fn}
}

func (j *RecJob) Run() *RecJob {
	if j.fn != nil {
		j.Err = j.fn(&j.Out)
		return j
	}
	c := exec.Command(j.argv[0], j.argv[1:]...)
	c.Stdout, c.Stderr = &j.Out, &j.Out
	j.Err = c.Run()
	return j
}

// recAll runs every job at once and waits for all of them.
func recAll(jobs ...*RecJob) {
	var wg sync.WaitGroup
	for _, j := range jobs {
		wg.Add(1)
		go func(j *RecJob) { defer wg.Done(); j.Run() }(j)
	}
	wg.Wait()
}

func (j *RecJob) Failed() bool { return j.Err != nil }

// tell prints one failed job: what it was, how it ended, what it last said.
func (j *RecJob) Tell(w io.Writer) {
	how := "could not be started: " + fmt.Sprint(j.Err)
	if _, ok := j.Err.(*exec.ExitError); ok {
		how = fmt.Sprintf("exited %d", ExitCode(j.Err))
	}
	what := j.how
	if what == "" {
		what = strings.Join(j.argv[1:], " ")
	} else {
		how = "failed: " + fmt.Sprint(j.Err)
	}
	fmt.Fprintf(w, "  %-12s %s did not finish -- `%s` %s.  It said:\n", "record", j.What, what, how)
	ls := Lines(strings.TrimRight(j.Out.String(), "\n"))
	if len(ls) == 0 {
		fmt.Fprintf(w, "               (nothing at all)\n")
	}
	if len(ls) > 12 {
		ls = ls[len(ls)-12:]
	}
	for _, l := range ls {
		fmt.Fprintf(w, "               %s\n", l)
	}
}

// recRefuse reports every failed job and says whether there was one.  A
// caller refuses on true: whatever it would compare next is a comparison
// with a recording that is not whole.
func RecRefuse(w io.Writer, jobs ...*RecJob) bool {
	bad := false
	for _, j := range jobs {
		if j.Failed() {
			j.Tell(w)
			bad = true
		}
	}
	if bad {
		fmt.Fprintf(w, "               A recording that did not finish is a reading of the machine or of the\n")
		fmt.Fprintf(w, "               phase, and the lines above are what tells which; nothing after it was run.\n")
	}
	return bad
}

// missingRecords is every record the baseline directory holds and the
// candidate does not -- which is what a recorder that died leaves, and what a
// comparison would otherwise count as MOVED.
func MissingRecords(base, cand string) []string {
	var Out []string
	es, _ := os.ReadDir(base)
	for _, e := range es {
		if _, err := os.Stat(cand + "/" + e.Name()); err != nil {
			Out = append(Out, e.Name())
		}
	}
	return Out
}

// recCmd is the drop-in for `exec.Command(argv...).Run()` at a recording site:
// the same run, with the output kept, and an error that carries it.  The
// recording is named by the directory it writes, which is its last argument.
func RecCmd(argv ...string) error {
	j := NewRec("the recording into "+recBase(argv[len(argv)-1]), argv...).Run()
	if j.Failed() {
		return &recError{j}
	}
	return nil
}

func recBase(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[i+1:]
	}
	return p
}

type recError struct{ j *RecJob }

func (e *recError) Error() string { return e.j.What + " did not finish" }

// recReport tells every failed recording among errs, and says whether any
// error at all is there -- the caller refuses on true, exactly where it used
// to refuse with nothing printed.
func RecReport(w io.Writer, errs ...error) bool {
	var jobs []*RecJob
	any := false
	for _, e := range errs {
		if e == nil {
			continue
		}
		any = true
		if re, ok := e.(*recError); ok {
			jobs = append(jobs, re.j)
		} else {
			fmt.Fprintf(w, "  %-12s a recording did not finish: %v\n", "record", e)
		}
	}
	RecRefuse(w, jobs...)
	return any
}
