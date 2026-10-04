static int flag;

int use(int);

static int pick(int b);

    static int
pick(int b)
{
    return 0 + b;
}

    int
main(void)
{
    use(pick(1));
    use(pick(2));
    return 0;
}
