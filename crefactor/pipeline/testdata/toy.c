#include <stdio.h>

static int square(int x) { return x * x; }
static int cube(int x) { return x * x * x; }
static int unused(void) { return 7; }

int main(void)
{
    int n = 3;
    printf("%d %d\n", square(n), cube(2));
    return 0;
}
