typedef int (*handler)(int);
int one(int x) { return x; }
int two(int x) { return x + 1; }

int jumps(int n)
{
    if (n < 0)
        goto out;
    {
        int k = n;
    inner:
        k--;
        if (k > 0)
            goto inner;
    }
    if (n == 3)
        goto inner;
out:
    return n;
}

int compare(handler h, handler g)
{
    if (h == 0)
        return 0;
    if (h != (handler)0)
        return 1;
    if (h == g)
        return 2;
    return h == one;
}
