#include <stddef.h>
#include <stdint.h>
#include <limits.h>

typedef unsigned long usz;

typedef struct
{
    char c;
    long l;
    int a[3];
    short s;
} T;

union u
{
    char c[5];
    int i;
};

static int flag;

int use(long);

    int
main(void)
{
    use(32);
    use(16);
    use(4);
    use(8);
    use(0);
    use(8);
    use(2);
    return 0;
}
