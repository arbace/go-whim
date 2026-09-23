package check

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/arbace/go-whim/internal/harness"
)

// whimEnv is whim80's and whim81's environment: the caller's, with an empty
// home and no vimrc from anywhere.
func WhimEnv(home string) []string {
	var env []string
	for _, kv := range os.Environ() {
		k := kv[:strings.IndexByte(kv, '=')]
		switch k {
		case "VIMINIT", "EXINIT", "MYVIMRC", "HOME", "VIM", "VIMRUNTIME", "XDG_CONFIG_HOME":
			continue
		}
		env = append(env, kv)
	}
	return append(env, "HOME="+home, "VIM="+home+"/novim", "VIMRUNTIME="+home+"/novim", "XDG_CONFIG_HOME="+home+"/xdg")
}

// whimRes is one headless run: its status, its stderr, what it left in its
// directory and one file's contents afterwards (nil when there is none).
type WhimRes struct {
	Rc      string
	Stderr  []byte
	left    []string
	Body    *string
	Timeout bool
}

func (a WhimRes) Eq(b WhimRes, withLeft bool) bool {
	if a.Timeout || b.Timeout {
		return a.Timeout == b.Timeout
	}
	if a.Rc != b.Rc || !bytes.Equal(a.Stderr, b.Stderr) {
		return false
	}
	if withLeft && strings.Join(a.left, "\x00") != strings.Join(b.left, "\x00") {
		return false
	}
	if (a.Body == nil) != (b.Body == nil) {
		return false
	}
	return a.Body == nil || *a.Body == *b.Body
}

// whimRun runs `vim -e -s <args> f.txt` in a fresh directory holding the
// three-line f.txt, and reads `file` back afterwards.
func WhimRun(vim, tmp string, env []string, argv []string, file string) WhimRes {
	w, err := os.MkdirTemp(tmp, "c-")
	if err != nil {
		return WhimRes{Rc: "MKDIR"}
	}
	defer os.RemoveAll(w)
	os.WriteFile(filepath.Join(w, "f.txt"), []byte("a\nba\nca\n"), 0o644)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, vim, append(append([]string{"-e", "-s"}, argv...), "f.txt")...)
	c.Dir, c.Env = w, env
	harness.Setsid(c)
	var so, se bytes.Buffer
	c.Stdout, c.Stderr = &so, &se
	e := c.Run()
	if ctx.Err() != nil {
		return WhimRes{Timeout: true, Rc: "TIMEOUT"}
	}
	rc := "0"
	if e != nil {
		rc = strconv.Itoa(ExitCode(e))
	}
	ents, _ := os.ReadDir(w)
	var left []string
	for _, x := range ents {
		left = append(left, x.Name())
	}
	sort.Strings(left)
	var Body *string
	if b, err := os.ReadFile(filepath.Join(w, file)); err == nil {
		s := strings.ToValidUTF8(string(b), "�")
		Body = &s
	}
	return WhimRes{Rc: rc, Stderr: se.Bytes(), left: left, Body: Body}
}

// whimPool runs fn over n items with one worker per CPU.
func WhimPool(n int, fn func(i int)) {
	sem := make(chan struct{}, runtime.NumCPU())
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			fn(i)
			<-sem
		}(i)
	}
	wg.Wait()
}
