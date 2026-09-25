#include <stdio.h>
#include <string.h>

enum color { RED, GREEN, BLUE, UNUSED_COLOR, YELLOW };

struct pair {
    int key;
    int value;
    int never_read;
};

struct point {
    int x;
    int y;
};

static struct point origin = { 3, 4 };

static int dead_counter;

static int unused_helper(int x)
{
    return x * dead_counter;
}

static int sum(const int *v, int n)
{
    int total = 0;
    int unused_local = 7;
    for (int i = 0; i < n; i++)
        total += v[i];
    return total;
}

static const char *color_name(enum color c)
{
    switch (c) {
    case RED: return "red";
    case GREEN: return "green";
    case BLUE: return "blue";
    case YELLOW: return "yellow";
    default: return "?";
    }
}

int main(void)
{
    int v[] = { 1, 2, 3, 4 };
    struct pair p;
    p.key = 1;
    p.value = sum(v, 4);
    printf("%d %d\n", p.key, p.value);
    printf("%s %d\n", color_name(YELLOW), (int)YELLOW);
    printf("%d %d\n", origin.x, origin.y);
    printf("%zu\n", strlen("foreign"));
    return 0;
}
