/* The core's stand-in for measuring a hypercall (doc/GUEST.md, milestone
 * 5): argv[1] calls of host_time, the cheapest call there is, then exit. */
static long host_time(void);

static int
vim_main(int argc, char **argv)
{
    long n = 0;
    if (argc > 1)
    {
        for (const char *p = argv[1]; *p >= '0' && *p <= '9'; p++)
        {
            n = n * 10 + (*p - '0');
        }
    }
    for (long i = 0; i < n; i++)
    {
        host_time();
    }
    return 0;
}

static void
deathtrap(int sigarg)
{
    (void)sigarg;
}
