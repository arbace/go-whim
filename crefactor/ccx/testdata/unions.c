struct value {
    int kind;
    union {
        long n;
        char *s;
    } u;
};

int mode;
int mode_save;

long as_number(struct value *v)
{
    if (v->kind == 1)
        return v->u.n;
    return 0;
}

char *as_string(struct value *v)
{
    if (v->kind != 1)
        return v->u.s;
    return v->u.s;
}

long only_numbers(struct value *v)
{
    return v->u.n;
}

long caller(struct value *v)
{
    if (v->kind == 1)
        return only_numbers(v);
    return 0;
}

long early(struct value *v)
{
    if (v->kind != 1)
        return 0;
    return v->u.n;
}

void set_mode(int m)
{
    mode = m;
}

void keeps_mode(void)
{
    mode_save = mode;
    set_mode(2);
    mode = mode_save;
}

long read_n(struct value *v)
{
    return v->u.n;
}

long by_mode(struct value *v)
{
    long r = 0;
    if (mode == 1) {
        keeps_mode();
        r = read_n(v);
    }
    return r;
}

long by_mode_badly(struct value *v)
{
    long r = 0;
    if (mode == 1) {
        set_mode(0);
        r = read_n(v);
    }
    return r;
}

long unguarded(struct value *v)
{
    return read_n(v);
}
