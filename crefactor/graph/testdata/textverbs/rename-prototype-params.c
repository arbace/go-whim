#include <stddef.h>

static int total = 0;

static int g(int lhs, int y);

    static int
g(int x, int y)
{
    return x + y;
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
    if (a > 1 && a < 9)
    {
        k = g(k, 2);
    }
    g(k, 3);
    total = k;
    return total;
}
