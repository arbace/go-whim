#include <stdio.h>

int ping(int n);
int pong(int n);

static int
helper(int x)
{
    return x * 2;
}

int
ping(int n)
{
    return n <= 0 ? 0 : pong(n - 1);
}

int
pong(int n)
{
    return n <= 0 ? 1 : ping(n - 1);
}

static int
orphan(void)
{
    return helper(3);
}

static int
on_red(void)
{
    return 1;
}

static int (*const handlers[])(void) = { on_red };

int
main(void)
{
    printf("%d %d\n", helper(4), handlers[0]());
    return 0;
}
