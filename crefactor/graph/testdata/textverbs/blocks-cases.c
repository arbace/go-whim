void f(void);

void g2(void);

    int
h(int a)
{
    if (a)
    {
        f();
    }
    {
        int x;
        g2();
    }
    a = a + 1;
    return a;
}

    int
sw(int c)
{
    int x = 0;
    switch (c)
    {
    case 'b':
        x = 2;
        break;
    case 'f':
        x = 4;
    case 'g':
        x = 5;
        break;
    default:
        x = 3;
    }
    if (c && x < 9)
    {
        x = 0;
    }
    return x;
}
