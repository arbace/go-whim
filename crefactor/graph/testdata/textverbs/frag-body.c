#include <errno.h>
#include <stddef.h>
#include <string.h>
#include <sys/param.h>
#include <stdarg.h>

enum
{
    FALSE,
    TRUE,
};

struct buf
{
    struct buf *next;
    int n;
};

struct win
{
    struct win *next;
    struct buf *w_buffer;
    int n;
};

typedef struct buf buf_T;

static buf_T *curbuf;

static int total = 0;

int g(int x);

static int sum(int n, ...);

    int
f(int a)
{
    struct win *wp = nullptr;
    int k = a + (int)strlen("x");
    if (k == 0)
    {
        goto out;
    }
    errno = 0;
    k = (int)__builtin_offsetof(struct win, n) + curbuf->next->n;
    if (wp != nullptr && wp->next->w_buffer == curbuf)
    {
        return wp->n;
    }
out:
    return g(k) + sum(2, k, total);
}

    int
g(int x)
{
    return x + total;
}

    static int
sum(int n, ...)
{
    va_list ap;
    int s = 0;
    va_start(ap, n);
    while (n-- > 0)
    {
        s += va_arg(ap, int);
    }
    va_end(ap);
    return s;
}

    int
pick(struct win *wp, int lo, int hi)
{
    int w = MIN(lo, hi);
    return MAX(w, wp->n) + MIN(hi, 3);
}
