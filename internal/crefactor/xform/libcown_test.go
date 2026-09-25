package xform

import (
	"strings"
	"testing"
)

// A core that asked libc for strlen takes it on as its own: the declaration
// goes, the calls are renamed outside the literal that names it, and the
// definition lands before its sibling's, above the host.
func TestOwn(t *testing.T) {
	k := OwnKnobs{
		Prefix: "my_",
		Funcs: []OwnFunc{{
			Name:  "strlen",
			Proto: "unsigned long strlen(const char *s);",
			Def:   "    static unsigned long\nmy_strlen(const char *s)\n{\n    unsigned long n = 0;\n    while (s[n])\n    {\n        n++;\n    }\n    return n;\n}\n\n",
		}},
		Before: "    static int\nmy_strcmp(",
	}
	src := `unsigned long strlen(const char *s);

int puts(const char *s);

    static int
my_strcmp(const char *a, const char *b)
{
    while (*a && *a == *b)
    {
        a++;
        b++;
    }
    return *a - *b;
}

int
width(const char *s)
{
    puts("width");
    return strlen(s) + strlen("..");
}

#include <stdio.h>

#include <string.h>
`
	// the declaration's line goes, and the blank after it stays
	want := `
int puts(const char *s);

    static unsigned long
my_strlen(const char *s)
{
    unsigned long n = 0;
    while (s[n])
    {
        n++;
    }
    return n;
}

    static int
my_strcmp(const char *a, const char *b)
{
    while (*a && *a == *b)
    {
        a++;
        b++;
    }
    return *a - *b;
}

int
width(const char *s)
{
    puts("width");
    return my_strlen(s) + my_strlen("..");
}

#include <stdio.h>

#include <string.h>
`
	got := run(t, Own(k), src, "strlen=2")
	same(t, got, want)
	refuses(t, Own(k), src, "told 3", "strlen=3")
	refuses(t, Own(k), strings.Replace(src, `puts("width");`, `puts("strlen");`, 1), "PRINTS")
	refuses(t, Own(k), strings.Replace(src, "#include <stdio.h>\n\n", "#include <stdio.h>\nint x;\n", 1), "contiguous")
}
