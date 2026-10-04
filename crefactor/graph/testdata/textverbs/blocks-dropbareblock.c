void f(void);

void g2(void);

    int
h(int a)
{
    if (a)
    {
        f();
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
    case 'a':
    case 'b':
        x = 2;
        break;
    case 'n':
        x = 1;
        break;
    case 'f':
        x = 4;
    case 'g':
        x = 5;
        break;
    default:
        x = 3;
    }
    if (c && x > 1 && x < 9)
    {
        x = 0;
    }
    return x;
}
