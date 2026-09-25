typedef unsigned long size_t;
void *pool_get(size_t n);
void copy_bytes(void *dst, const void *src, size_t n);

typedef struct vec {
    int len;
    int itemsize;
    void *data;
} vec_T;

void vec_init(vec_T *g, int size)
{
    g->len = 0;
    g->itemsize = size;
    g->data = 0;
}

void vec_grow(vec_T *g, int n)
{
    void *p = pool_get((size_t)(g->len + n) * (size_t)g->itemsize);
    if (g->data != 0)
        copy_bytes(p, g->data, (size_t)g->len * (size_t)g->itemsize);
    g->data = p;
}

vec_T names;
vec_T nums;

void put_int(vec_T *g, int x)
{
    vec_grow(g, 1);
    ((int *)g->data)[g->len++] = x;
}

void setup(void)
{
    vec_init(&names, sizeof(char *));
    vec_init(&nums, sizeof(int));
    vec_grow(&names, 1);
    ((char **)names.data)[0] = "one";
    put_int(&nums, 2);
}
