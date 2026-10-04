#include <stddef.h>

static int total = 0;

static int g(int x, int y);

    static int
g(int x, int y)
{
    return x - y;
}

static int seen;

    static void
h(void)
{
    total = 1;
}

    int
f(int a)
{
    int k = a;
    if (total == 0 && a > 1 && a < 9)
    {
        k = g(k, 2);
        total++;
    }
    if (k)
    {
        g(k, 3);
    }
    total--;
    total = k;
    return total;
}
