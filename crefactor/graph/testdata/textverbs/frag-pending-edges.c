#include <stddef.h>

static int total = 0;

static int seen = 0;

static int g(int x, int y);

    static int
g(int x, int y)
{
    return x + y;
}

    static void
mark(void)
{
    seen = 1;
}

    static void
h(void)
{
    total = 1;
}

    int
f(int a)
{
    int k = a;
    int twice = k * 2;
    if (a > 1 && a < 9)
    {
        k = g(k, 2);
    }
    g(k, 3);
    mark();
    total = twice;
    return total;
}
