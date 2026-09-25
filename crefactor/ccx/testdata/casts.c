typedef unsigned long size_t;
void *pool_get(size_t n);
void pool_put(void *p);
void *copy_bytes(void *dst, const void *src, size_t n);

struct rect { int w, h; };
struct circle { int r; };

int area(int (*f)(int), int x);
int twice(int x) { return 2 * x; }

void casts(struct rect *r, char *s, void *v)
{
    struct rect *a = (struct rect *)pool_get(sizeof(struct rect));
    struct circle *c = (struct circle *)r;
    unsigned char *u = (unsigned char *)s;
    struct rect *same = (struct rect *)r;
    char *b = (char *)v;
    int *z = (int *)0;
    long n = 5;
    int *bad = (int *)n;
    copy_bytes((char *)r, (char *)a, sizeof(struct rect));
    pool_put((void *)c);
    area((int (*)(int))twice, 1);
    (void)u; (void)same; (void)b; (void)z; (void)bad;
}
