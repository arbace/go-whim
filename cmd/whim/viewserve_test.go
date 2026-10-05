package main

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"testing"
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
	go func() { serveViews(g, viewSample, inR, outW); outW.Close() }()
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
