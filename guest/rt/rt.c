/* The guest runtime: what stands below the core's line when the core is a
 * virtual machine's only code (doc/GUEST.md).  The core's 17 host functions
 * are defined here -- 15 as hypercalls, host_alloc and host_free over the
 * guest's own RAM -- with whim_main, which the entry stub calls, and
 * whim_fault, which the exception vectors call.
 *
 * It is compiled in ONE translation unit after the core (and the C host's
 * vim_snprintf, which needs no system), because the core declares its host
 * functions static.  A program standing in for the core (hello.c) declares
 * vim_main and deathtrap itself.
 *
 * A hypercall: the call block below filled, then its guest-physical address
 * (the guest is identity-mapped) stored to the doorbell, a page no memory
 * backs, which exits to the monitor.  The store is one instruction from a
 * fixed register -- RAX on amd64, X0 on arm64 -- so that a backend that is
 * not told which register the store read (KVM) can say it.  The monitor
 * writes ret, any out-parameters into a[], and event: a deadly signal it
 * caught, which the core's deathtrap() takes here, as the C host's signal
 * handler would have at the call. */

typedef typeof(sizeof(0)) whim_usize;

void *memcpy(void *dst, const void *src, whim_usize n);

/* The core's linkage, and its host functions': static, in the one
 * translation unit the core is compiled in. */
#define WHIM_CORE static

WHIM_CORE int vim_main(int argc, char **argv);
WHIM_CORE void deathtrap(int sigarg);

enum
{
    WHIM_DOORBELL = 0xf0000000,
    WHIM_PORT = 0x5157,          /* the alternative trap's, amd64 */
    WHIM_HVC_FN = 0xc3000057,    /* and arm64's: an SMC64 fast call, OEM service */
    WHIM_HEAP_BYTES = 1024 * 1024 * 1024, /* the C host's arena */
};

enum whim_nr
{
    WHIM_HOST_INIT = 1,
    WHIM_GET_WINSIZE,
    WHIM_TERM_START,
    WHIM_TERM_STOP,
    WHIM_TTY_KEYS,
    WHIM_NOW_MS,
    WHIM_DELAY,
    WHIM_WAIT_FOR_INPUT,
    WHIM_READ_INPUT,
    WHIM_SUSPEND,
    WHIM_EXIT,
    WHIM_MESSAGE,
    WHIM_ALLOC, /* never made: answered in the guest */
    WHIM_FREE,  /* never made: answered in the guest */
    WHIM_WRITE,
    WHIM_TIME,
    WHIM_RAISE,
    WHIM_FAULT, /* the runtime's own: an exception the vectors took */
    WHIM_RANDOM, /* a Go guest's; never made here */
    WHIM_WAIT_READ, /* the wait and the read merged: guest/abi's WaitRead */
};

struct whim_call
{
    unsigned long nr;
    long a[5];
    long ret;
    unsigned long event;
};

static volatile struct whim_call whim_call __attribute__((aligned(64)));

static char *whim_heap;

static void
whim_doorbell(void)
{
#if defined(WHIM_TRAP_ALT) && defined(__x86_64__)
    /* the alternative (doc/GUEST.md, milestone 5): an out to a port */
    __asm__ volatile("outl %%eax, %%dx" : : "a"((unsigned)(unsigned long)&whim_call), "d"((unsigned short)WHIM_PORT) : "memory");
#elif defined(WHIM_TRAP_ALT) && defined(__aarch64__)
    /* the alternative: an HVC, an SMCCC call KVM forwards to the monitor */
    register long x0 __asm__("x0") = WHIM_HVC_FN;
    register volatile struct whim_call *x1 __asm__("x1") = &whim_call;
    __asm__ volatile("hvc #0" : "+r"(x0) : "r"(x1) : "memory", "x2", "x3");
#elif defined(__x86_64__)
    __asm__ volatile("movq %%rax, (%1)" : : "a"(&whim_call), "r"((unsigned long)WHIM_DOORBELL) : "memory");
#elif defined(__aarch64__)
    register volatile struct whim_call *x0 __asm__("x0") = &whim_call;
    __asm__ volatile("str x0, [%1]" : : "r"(x0), "r"((unsigned long)WHIM_DOORBELL) : "memory");
#else
#error "an ISA the guest has no doorbell for"
#endif
}

/* whim_hcall makes call nr.  A deadly signal the monitor caught comes back
 * as event: deathtrap() runs, as the C handler would have; when it returns
 * (the signal blocked), a wait or a read is made again, as the C host's
 * select and read went on after their EINTR. */
static long
whim_hcall(enum whim_nr nr, long a0, long a1, long a2, long a3, long a4)
{
    for (;;)
    {
        whim_call.nr = nr;
        whim_call.a[0] = a0;
        whim_call.a[1] = a1;
        whim_call.a[2] = a2;
        whim_call.a[3] = a3;
        whim_call.a[4] = a4;
        whim_call.ret = 0;
        whim_call.event = 0;
        whim_doorbell();
        if (whim_call.event != 0)
        {
            deathtrap((int)whim_call.event);
            if (nr == WHIM_WAIT_FOR_INPUT || nr == WHIM_READ_INPUT || nr == WHIM_WAIT_READ)
            {
                continue;
            }
        }
        return whim_call.ret;
    }
}

WHIM_CORE void
musl_host_init(void)
{
    whim_hcall(WHIM_HOST_INIT, 0, 0, 0, 0, 0);
}

WHIM_CORE int
musl_get_winsize(int *rows, int *cols)
{
    int ok = (int)whim_hcall(WHIM_GET_WINSIZE, 0, 0, 0, 0, 0);
    if (ok)
    {
        *rows = (int)whim_call.a[0];
        *cols = (int)whim_call.a[1];
    }
    return ok;
}

WHIM_CORE void
musl_term_start(void)
{
    whim_hcall(WHIM_TERM_START, 0, 0, 0, 0, 0);
}

WHIM_CORE void
musl_term_stop(void)
{
    whim_hcall(WHIM_TERM_STOP, 0, 0, 0, 0, 0);
}

WHIM_CORE int
musl_tty_keys(int fd, int *bs, int *intr, int *cr, int *nlcr)
{
    int ok = (int)whim_hcall(WHIM_TTY_KEYS, fd, 0, 0, 0, 0);
    if (ok)
    {
        *bs = (int)whim_call.a[0];
        *intr = (int)whim_call.a[1];
        *cr = (int)whim_call.a[2];
        *nlcr = (int)whim_call.a[3];
    }
    return ok;
}

WHIM_CORE long
musl_now_ms(void)
{
    return whim_hcall(WHIM_NOW_MS, 0, 0, 0, 0, 0);
}

WHIM_CORE void
musl_delay(long ms, int interruptible)
{
    whim_hcall(WHIM_DELAY, ms, interruptible, 0, 0, 0);
}

/* The wait and the read, merged (doc/GUEST.md, *The merged wait and read*).
 * A wait is one call, WHIM_WAIT_READ: the monitor waits, and when there is
 * input it reads it at once into whim_input, as the core's read would have
 * -- WHIM_INPUT_BYTES, the core's INBUFLEN, the most its one read asks (in
 * fill_input_buf: INBUFLEN less what its own inbuf holds, which is nothing
 * after a wait in WaitForChar; guest.Source holds the two equal).  The
 * read's answer is kept, and the core's next read_input is served from it
 * with no exit: the bytes, as many as it asks, or the 0 or -1 the read
 * returned.  While any of it is kept a wait answers 1 here, as the C's
 * select would for bytes in the tty; once it is all the core's, a read
 * makes its own call again, as before.  A signal the monitor caught before
 * or during the call comes back as it did: a deadly one as event (before a
 * byte is read: the call is made again), one the editor reads as input as
 * its key sequence, which the host's read writes ahead of the tty's bytes. */
enum
{
    WHIM_INPUT_BYTES = 250,
};

static char whim_input[WHIM_INPUT_BYTES];
static int whim_input_kept;  /* a read's answer is kept: the core's next read is it */
static long whim_input_ret;  /* what is left of it: the count not yet the core's, or 0 or -1 */
static int whim_input_at;    /* where in whim_input that count starts */

WHIM_CORE int
musl_wait_for_input(long ms)
{
    if (whim_input_kept)
    {
        return 1;
    }
    if (whim_hcall(WHIM_WAIT_READ, ms, (long)whim_input, WHIM_INPUT_BYTES, 0, 0) == 0)
    {
        return 0;
    }
    whim_input_kept = 1;
    whim_input_ret = whim_call.a[0];
    whim_input_at = 0;
    return 1;
}

WHIM_CORE int
musl_read_input(char *buf, int len)
{
    long n;
    if (!whim_input_kept || len <= 0)
    {
        return (int)whim_hcall(WHIM_READ_INPUT, (long)buf, len, 0, 0, 0);
    }
    if (whim_input_ret <= 0)
    {
        whim_input_kept = 0;
        return (int)whim_input_ret;
    }
    n = whim_input_ret < len ? whim_input_ret : len;
    memcpy(buf, whim_input + whim_input_at, (whim_usize)n);
    whim_input_at += (int)n;
    whim_input_ret -= n;
    whim_input_kept = whim_input_ret > 0;
    return (int)n;
}

WHIM_CORE void
musl_suspend(void)
{
    whim_hcall(WHIM_SUSPEND, 0, 0, 0, 0, 0);
}

WHIM_CORE void
host_exit(int r)
{
    whim_hcall(WHIM_EXIT, r, 0, 0, 0, 0);
    for (;;)
    {
        whim_doorbell(); /* the monitor never resumes an exit */
    }
}

WHIM_CORE void
host_message(const char *msg, int len, int err)
{
    if (len < 0)
    {
        len = 0;
        while (msg[len] != '\0')
        {
            len++;
        }
    }
    whim_hcall(WHIM_MESSAGE, (long)msg, len, err, 0, 0);
}

WHIM_CORE int
host_write(const char *s, int len)
{
    if (len < 0)
    {
        return -1;
    }
    if (len == 0)
    {
        return 0;
    }
    return (int)whim_hcall(WHIM_WRITE, (long)s, len, 0, 0, 0);
}

WHIM_CORE long
host_time(void)
{
    return whim_hcall(WHIM_TIME, 0, 0, 0, 0, 0);
}

WHIM_CORE void
host_raise(int sig)
{
    whim_hcall(WHIM_RAISE, sig, 0, 0, 0, 0);
}

/* The allocator is the C host's, over the heap the monitor hands the entry:
 * a bump arena of 1 GiB that never frees, its memory zero as the C's static
 * arena is, its exhaustion reported in the C's words.  The monitor's pages
 * are committed on first touch, so the arena costs what is used. */
static whim_usize host_arena_used;

static int
whim_say(char *b, int at, const char *s)
{
    while (*s != '\0')
    {
        b[at++] = *s++;
    }
    return at;
}

static int
whim_num(char *b, int at, unsigned long v, int base)
{
    char d[24];
    int i = 24;
    if (v == 0)
    {
        d[--i] = '0';
    }
    while (v > 0)
    {
        d[--i] = "0123456789abcdef"[v % (unsigned long)base];
        v /= (unsigned long)base;
    }
    while (i < 24)
    {
        b[at++] = d[i++];
    }
    return at;
}

WHIM_CORE void *
host_alloc(whim_usize n)
{
    whim_usize want = (n + 15) & ~(whim_usize)15;
    char *p;
    if (want < n || want > WHIM_HEAP_BYTES - host_arena_used)
    {
        char m[160];
        int at = 0;
        at = whim_say(m, at, "whim-vim: host arena exhausted: ");
        at = whim_num(m, at, WHIM_HEAP_BYTES, 10);
        at = whim_say(m, at, " bytes, ");
        at = whim_num(m, at, host_arena_used, 10);
        at = whim_say(m, at, " used, request ");
        at = whim_num(m, at, n, 10);
        at = whim_say(m, at, "\n");
        host_message(m, at, 1);
        host_exit(1);
    }
    p = whim_heap + host_arena_used;
    host_arena_used += want;
    return p;
}

WHIM_CORE void
host_free(void *p)
{
    (void)p;
}

/* What the compiler may call for a struct copy or a zeroed array. */
void *
memcpy(void *dst, const void *src, whim_usize n)
{
    char *d = dst;
    const char *s = src;
    while (n-- > 0)
    {
        *d++ = *s++;
    }
    return dst;
}

void *
memset(void *dst, int c, whim_usize n)
{
    char *d = dst;
    while (n-- > 0)
    {
        *d++ = (char)c;
    }
    return dst;
}

int
memcmp(const void *a, const void *b, whim_usize n)
{
    const unsigned char *p = a, *q = b;
    for (whim_usize i = 0; i < n; i++)
    {
        if (p[i] != q[i])
        {
            return p[i] < q[i] ? -1 : 1;
        }
    }
    return 0;
}

int
bcmp(const void *a, const void *b, whim_usize n)
{
    return memcmp(a, b, n);
}

whim_usize
strlen(const char *s)
{
    whim_usize n = 0;
    while (s[n] != 0)
    {
        n++;
    }
    return n;
}

void *
memmove(void *dst, const void *src, whim_usize n)
{
    char *d = dst;
    const char *s = src;
    if (d < s)
    {
        while (n-- > 0)
        {
            *d++ = *s++;
        }
    }
    else
    {
        while (n-- > 0)
        {
            d[n] = s[n];
        }
    }
    return dst;
}

/* whim_fault is where an exception the core caused ends: reported by a
 * hypercall of its own, which the monitor turns into the C editor's death
 * by SIGSEGV. */
__attribute__((used, noreturn)) void
whim_fault(unsigned long vector, unsigned long code, unsigned long pc, unsigned long addr)
{
    for (;;)
    {
        whim_hcall(WHIM_FAULT, (long)vector, (long)code, (long)pc, (long)addr, 0);
    }
}

/* whim_main is where the entry stub goes, with the command line the monitor
 * wrote into guest memory and the heap it set aside. */
__attribute__((used, noreturn)) void
whim_main(long argc, char **argv, char *heap)
{
    whim_heap = heap;
    host_exit(vim_main((int)argc, argv));
    for (;;)
    {
    }
}
