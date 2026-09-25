int counter;
int bump(void) { return ++counter; }
int peek(int x) { int y = x; y++; return y; }
int ext_len(const char *s);

int f(int a, int b) { return a + b; }

void order(int i, int *v, const char *s)
{
    int x = bump() + bump();
    int y = peek(i) + bump();
    int z = ext_len(s) + bump();
    v[i] = i++;
    f(i++, i);
    int w = (i++, i);
    int q = bump() && bump();
    (void)x; (void)y; (void)z; (void)w; (void)q;
}
