static int flag;

int use(int);

int g;

    int
main(void)
{
    int x = use(0);
    if (x)
    {
        use(1);
    }
    if (x)
    {
        use(2);
    }
    if (use(3) && 0)
    {
        use(4);
    }
    g = x && 1;
    g = (x > 1) && 1;
    if (x && g)
    {
        use(5);
    }
    return 0;
}
