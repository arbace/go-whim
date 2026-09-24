package p119

// internal/phase/119/edit.go's own three literals, EXTRACTED by an AST walk rather
// than retyped: the two declarations the phase takes Out of the core's block,
// the prototype it adds at the end of the core -> host run, and the definition
// it puts INSIDE the host region.
var w119Go = []string{"int getpid(void);", "int kill(int pid, int sig);"}

const (
	w119Proto = "static void host_raise(int sig);"
	w119Def   = "    static void\nhost_raise(int sig)\n{\n    kill(getpid(), sig);\n}\n\n"
)
