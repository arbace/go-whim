struct st
{
    int on;
    int n;
};

static struct st s;

static struct st *sp = &s;

int use(int);

    int
main(void)
{
    sp->n = 3;
    return s.n;
}
