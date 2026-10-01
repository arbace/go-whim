package xform

import (
	"bytes"
	"strings"
	"testing"
)

const falloutC = `void *memset(void *, int, unsigned long);
int puts(const char *);
typedef struct
{
    int argc;
    int quiet;
    char *rc;
    int extra[4];
} parm_T;
static parm_T params;
static int never;
static int written;
static int was_quiet(void)
{
    return params.quiet;
}
static void init(int clean)
{
    if (clean)
    {
        puts("clean");
    }
    puts("init");
}
static void pre(parm_T *p)
{
    int n = p->extra[1];
    if (n <= 0)
    {
        return;
    }
    puts("pre");
}
static void scan(parm_T *p, int argc)
{
    p->argc = argc;
}
int main(int argc, char **argv)
{
    memset(&params, 0, sizeof(params));
    scan(&params, argc);
    written = argc;
    init(params.quiet);
    pre(&params);
    if (argc > 1 && !was_quiet())
    {
        puts("loud");
    }
    else if (never)
    {
        puts("never");
    }
    if (params.rc != 0 && written)
    {
        puts(params.rc);
    }
    return params.argc + written;
}
`

func TestFallOut(t *testing.T) {
	var log bytes.Buffer
	out, err := FallOut()([]byte(falloutC), nil, &log)
	if err != nil {
		t.Fatalf("%v\n%s", err, log.String())
	}
	s := string(out)
	t.Logf("%s\n%s", log.String(), s)
	for _, gone := range []string{"was_quiet()", "never)", "params.quiet", "params.rc", "init(params", "pre(&params)", "if (clean)", "fallout", "\"clean\""} {
		if strings.Contains(s, gone) {
			t.Errorf("%q survives", gone)
		}
	}
	for _, kept := range []string{"if (argc > 1)", "puts(\"loud\");", "scan(&params, argc);", "return params.argc + written;", "init();", "init(void)"} {
		if !strings.Contains(s, kept) {
			t.Errorf("%q is gone", kept)
		}
	}
	if _, err := translate(out); err != nil {
		t.Errorf("the result does not type-check: %v", err)
	}
}
