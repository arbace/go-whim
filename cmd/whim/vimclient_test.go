package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestVimClient: vim/'s client (doc/GRAPH.md, *The vim client*) driven by
// a headless vim against whim view-serve on the views' sample: a view
// opened, an operand erased (pending) and retyped (applied), a statement
// typed in on a line of its own (applied, laid out by the print), both
// undone to the text opened, the view at the cursor, and a definition's
// C in a split.  Skipped where there is no vim with +job.
func TestVimClient(t *testing.T) {
	vim, err := exec.LookPath("vim")
	if err != nil {
		t.Skip("no vim")
	}
	if out, _ := exec.Command(vim, "--version").Output(); !strings.Contains(string(out), "+job") {
		t.Skip("vim without +job")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "whim")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	sample, _ := filepath.Abs(viewSample)
	rtp, _ := filepath.Abs("../../vim")
	outFile := filepath.Join(dir, "out")
	script := `set nocompatible
let &rtp = '` + rtp + `,' .. &rtp
runtime plugin/whimview.vim
let g:whimview_cmd = ['` + bin + `', 'view-serve', '--no-cache', '` + sample + `']
let g:whimview_live = 0
let out = []
try
  WhimView def get
  let opened = whimview#State().text
  call add(out, 'open ' .. (opened =~ '(+= (-> b b_ml) 2)'))
  call search('(+= (-> b b_ml) \zs2')
  normal! x
  WhimSync
  call add(out, 'erase ' .. whimview#State().status)
  normal! i3
  WhimSync
  call add(out, 'retype ' .. whimview#State().status .. ' ' .. (whimview#State().text =~ '(+= (-> b b_ml) 3)'))
  call search('(post++')
  normal! O(+= opt 1)
  WhimSync
  call add(out, 'line ' .. whimview#State().status .. ' ' .. (getline(line('.')) == '  (+= opt 1)'))
  WhimUndo
  WhimUndo
  call add(out, 'undone ' .. (whimview#State().text == opened))
  call search('(= \zsopt 0)')
  WhimAt uses
  call add(out, 'at ' .. (whimview#State().text =~ '(uses opt'))
  call search('(= \zsopt')
  normal! x
  WhimSync
  try
    WhimView def main
    call add(out, 'switched with text pending')
  catch /pending/
    call add(out, 'pending kept')
  endtry
  WhimView! def main
  call search('(call \zsfact 3)')
  WhimRename factorial
  call add(out, 'rename ' .. (whimview#State().text =~ '(call factorial 3)'))
  WhimUndo
  WhimC get
  call add(out, 'c ' .. (join(getline(1, '$'), "\n") =~ 'b->b_ml += 2;'))
  WhimStop
catch
  call add(out, 'error ' .. v:exception .. ' ' .. v:throwpoint)
endtry
call writefile(out, '` + outFile + `')
qa!
`
	sf := filepath.Join(dir, "test.vim")
	if err := os.WriteFile(sf, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(vim, "-Nu", "NONE", "-i", "NONE", "-es", "-S", sf)
	cmd.Env = append(os.Environ(), "TMPDIR="+dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Logf("vim: %v\n%s", err, out)
	}
	got, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("vim wrote nothing: %v", err)
	}
	want := "open 1\nerase pending: \nretype applied 1\nline applied 1\nundone 1\nat 1\npending kept\nrename 1\nc 1\n"
	g := string(got)
	// the pending reason is the server's: compare up to it
	lines := strings.Split(g, "\n")
	for i, l := range lines {
		if strings.HasPrefix(l, "erase pending: ") {
			lines[i] = "erase pending: "
		}
	}
	if strings.Join(lines, "\n") != want {
		t.Fatalf("the client's session:\n%s\nwant:\n%s", g, want)
	}
	t.Logf("\n%s", g)
}
