static int flag;

int use(int);

    int
main(void)
{
    int i = use(0);
    switch (i)
    {
    case 1:
        ;
        break;
    case 2:
        ;
        break;
    }
    if (i)
    {
        use(4);
    }
    if (i)
    {
        use(5);
    }
    return 0;
}
