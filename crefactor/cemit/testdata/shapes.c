/* A small program in no particular layout: every construct the printer has
   one spelling for, written the way macro residue writes it. */
#include <stdio.h>

typedef struct   point { int x ,y; } point_t;   // trailing comment
union  cell { int i; char c [4]; };
enum shade { DARK , LIGHT = 3 , PALE };
static int table [ 3 ] = { 1 ,2, 3 };
static const char *names[] = { "a  b", "c/*not a comment*/", 0 };
static point_t origin = { .x = 1, .y = 2 };
static int (*pick)(int, int);

static int add (int p , int q) { return p+q; }
static int mul(int p, int q) { return p * q ; }

static int
classify(int v)
{
  if(v<0) return -1; else if (v==0) return 0;
  else { return 1; }
}

static int
loops(int n)
{
    int s = 0, i;
    for (i = 0; i < n; i++) s += i;
    while (n > 0) { n--; s++; }
    do s--; while (s > 100);
    for (;;) break;
    switch (n) { case 0: s += 1; break; case 1: case 2: s += 2; /* fall */ default: s += 3; }
    return s;
}

int
main(void)
{
    union cell u; u.i = 0; u.c[0] = 'z';
    enum shade sh = PALE;
    long wide = (long) origin.x + (unsigned char) 300;
    pick = sh == PALE ? add : mul;
    printf("%d %d %d %d %s %s %ld %d\n", classify(-5), classify(0), loops(5),
           pick(table[1], table[2]), names[0], names[1], wide, (int) sizeof (point_t));
    return u.c[0] == 'z' ? 0 : 1;
}
