typedef struct buf { int b_ml; struct buf *next; } buf_T;
struct other { buf_T *owner; int b_ml; };
static int opt;
enum { A, B };
static int get(buf_T *b, int n);
static int fact(int n) { if (n < 2) return 1; return n * fact(n - 1); }
static int even(int n);
static int odd(int n) { return n == 0 ? 0 : even(n - 1); }
static int even(int n) { return n == 0 ? 1 : odd(n - 1); }
static int get(buf_T *b, int n)
{
    b->b_ml = n;
    b->b_ml++;
    b->b_ml += 2;
    if (b->next)
    {
        opt = 0;
        return 0;
    }
    return b->b_ml + opt + A;
}
static buf_T *mk(void) { static buf_T one; return &one; }
static int (*table[])(int) = { fact, odd };
int main(void)
{
    buf_T b;
    struct other o;
    o.b_ml = 1;
    int *p = &b.b_ml;
    opt = 1;
    get(&b, fact(3));
    return b.b_ml + (int)sizeof(b.b_ml) + odd(*p) + o.b_ml + mk()->b_ml + table[0](2);
}
