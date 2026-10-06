package main

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/arbace/go-whim/crefactor/graph/view"
	"github.com/arbace/go-whim/internal/whim"
)

// TestViewServe: the editing server on the views' sample, driven as an
// editor drives it -- requests a line, answers framed -- through a view
// opened, a change pending then applied, the view at a cursor, undo, the
// C, and errors that leave the server answering.
func TestViewServe(t *testing.T) {
	g, _, err := loadGraph(viewSample, true)
	if err != nil {
		t.Fatal(err)
	}
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	fo := whim.GraphFallOut
	go func() { view.NewServer(g, viewSample, view.ServerOptions{FallOut: &fo}).Serve(inR, outW); outW.Close() }()
	rd := bufio.NewReader(outR)
	ask := func(req string) (kind, head, body string) {
		t.Helper()
		if _, err := fmt.Fprintln(inW, req); err != nil {
			t.Fatal(err)
		}
		h, err := rd.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		f := strings.SplitN(strings.TrimSuffix(h, "\n"), " ", 3)
		n, err := strconv.Atoi(f[1])
		if err != nil {
			t.Fatalf("header %q", h)
		}
		b := make([]byte, n)
		if _, err := io.ReadFull(rd, b); err != nil {
			t.Fatal(err)
		}
		end, _ := rd.ReadString('\n')
		if end == "\n" {
			end, _ = rd.ReadString('\n')
		}
		if end != ";;end\n" {
			t.Fatalf("%s: no ;;end after the body, %q", req, end)
		}
		if len(f) == 3 {
			head = f[2]
		}
		return f[0], head, string(b)
	}

	if k, _, b := ask("ping"); k != "ok" || b != "pong\n" {
		t.Fatalf("ping: %s %q", k, b)
	}
	if k, _, _ := ask("change 0 0 x"); k != "error" {
		t.Fatal("a change with no buffer")
	}
	k, _, text := ask("open def get")
	if k != "ok" || !strings.HasPrefix(text, "(defn static get") {
		t.Fatalf("open: %s\n%s", k, text)
	}
	at := strings.Index(text, "(+= (-> b b_ml) 2)") + len("(+= (-> b b_ml) ")
	_, head, _ := ask(fmt.Sprintf("change %d %d ''", at, at+1))
	if !strings.HasPrefix(head, "pending ") {
		t.Fatalf("an operand erased: %s", head)
	}
	_, head, body := ask(fmt.Sprintf("change %d %d \"3\"", at, at))
	if !strings.HasPrefix(head, "applied ") || !strings.Contains(body, "(+= (-> b b_ml) 3)") {
		t.Fatalf("typed: %s\n%s", head, body)
	}
	if _, head, _ := ask("edits"); head != "1" {
		t.Fatalf("edits: %s", head)
	}
	if _, _, c := ask("c get"); !strings.Contains(c, "b->b_ml += 3;") {
		t.Fatalf("the C:\n%s", c)
	}
	// the view at a cursor on the use of opt: its uses
	_, head, body = ask(fmt.Sprintf("at %d uses", strings.Index(body, "(= opt 0)")+len("(= ")))
	if !strings.HasSuffix(head, " opt") || !strings.HasPrefix(strings.TrimLeft(body, ";- 0123456789abcdefghijklmnopqrstuvwxyz:,\n"), "(uses opt") {
		t.Fatalf("at: %s\n%s", head, body)
	}
	if _, _, body = ask("open def get"); !strings.Contains(body, "(+= (-> b b_ml) 3)") {
		t.Fatalf("reopened:\n%s", body)
	}
	if _, _, body = ask("undo"); !strings.Contains(body, "(+= (-> b b_ml) 2)") {
		t.Fatalf("undone:\n%s", body)
	}
	if k, _, _ := ask("undo"); k != "error" {
		t.Fatal("an undo past the first edit")
	}
	// a function typed beside a def view's, reported; the viewed function
	// renamed at its name, its view still open (its root pinned by id)
	_, _, text = ask("open def fact")
	end := len(strings.TrimRight(text, "\n"))
	_, head, _ = ask(fmt.Sprintf(`change %d %d "\n(defn static twice (fn ((n int)) int) (return (* 2 (call fact n))))"`, end, end))
	if !strings.HasPrefix(head, "applied ") || !strings.HasSuffix(head, " added twice") {
		t.Fatalf("a function added beside: %s", head)
	}
	_, head, body = ask(fmt.Sprintf("rename %d factorial", strings.Index(text, "fact ")))
	if !strings.Contains(head, "declarations") || !strings.HasPrefix(body, "(defn static factorial") {
		t.Fatalf("renamed: %s\n%s", head, body)
	}
	if _, _, c := ask("c twice"); !strings.Contains(c, "factorial(n)") {
		t.Fatalf("twice after the rename:\n%s", c)
	}
	if k, _, _ := ask("no-such-request"); k != "error" {
		t.Fatal("a bad request")
	}
	if k, _, _ := ask(`change 0 0 "unclosed`); k != "error" {
		t.Fatal("an unclosed quote")
	}
	if k, _, _ := ask("quit"); k != "ok" {
		t.Fatal("quit")
	}
	inW.Close()
	io.Copy(io.Discard, outR)
}

// TestViewServeBuffers: two buffers on one session -- numbered by open,
// named by @B; an edit through one prints the other again, `touched`; the
// buffers listed; one closed, the other still answering.
func TestViewServeBuffers(t *testing.T) {
	g, _, err := loadGraph(viewSample, true)
	if err != nil {
		t.Fatal(err)
	}
	sv := view.NewServer(g, viewSample, view.ServerOptions{})
	ask := func(req string) (string, string) {
		t.Helper()
		head, body, err := sv.Answer(req)
		if err != nil {
			t.Fatalf("%s: %v", req, err)
		}
		return head, body
	}
	h1, get := ask("open def get")
	h2, _ := ask("open uses opt")
	if h1 != "buf 1" || h2 != "buf 2" {
		t.Fatalf("open's headers: %q %q", h1, h2)
	}
	at := strings.Index(get, "(= opt 0)") + len("(= opt ")
	head, _ := ask(fmt.Sprintf(`@1 change %d %d "2"`, at, at+1))
	if !strings.HasPrefix(head, "applied ") || !strings.HasSuffix(head, "touched 2") {
		t.Fatalf("an edit through buffer 1: %q", head)
	}
	if _, uses := ask("@2 text"); !strings.Contains(uses, "(= opt 2)") {
		t.Fatalf("buffer 2 not printed again:\n%s", uses)
	}
	if _, list := ask("buffers"); list != "1 def get\n2 uses opt\n" {
		t.Fatalf("buffers: %q", list)
	}
	ask("@1 close")
	if _, _, err := sv.Answer("@1 text"); err == nil {
		t.Fatal("a closed buffer answered")
	}
	if head, _ := ask("@2 undo"); head != "" {
		t.Fatalf("an undo with no other buffer: %q", head)
	}
}
