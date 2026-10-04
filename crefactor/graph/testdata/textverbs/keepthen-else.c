void h(int);

    void
g(int a)
{
    h(1);
    if (a > 1)
    {
        h(3);
    }
    else if (a > 2)
    {
        h(4);
    }
    else
    {
        h(5);
    }
}
