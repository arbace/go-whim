/* The core's stand-in for milestone 1 (doc/GUEST.md): "hello" written by
 * host_write and the exit code back by host_exit, the rest of the runtime
 * unused. */
static int host_write(const char *s, int len);

static int
vim_main(int argc, char **argv)
{
    (void)argc;
    (void)argv;
    host_write("hello\n", 6);
    return 3;
}

static void
deathtrap(int sigarg)
{
    (void)sigarg;
}
