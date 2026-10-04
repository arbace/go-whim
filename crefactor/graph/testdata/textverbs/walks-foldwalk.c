struct buf
{
    struct buf *next;
    int n;
};

static struct buf *firstbuf;

static struct buf *curbuf;

    int
walk(void)
{
    struct buf *bp;
    int total = 0;
    bp = curbuf;
    int k = bp->n;
    total += k;
    return total;
}

    int
escapes(void)
{
    struct buf *bp;
    for (bp = firstbuf; bp != 0; bp = bp->next)
    {
        if (bp->n)
        {
            break;
        }
        switch (bp->n)
        {
        case 1:
            break;
        }
    }
    return 0;
}

    int
clashes(void)
{
    struct buf *bp;
    int k = 0;
    for (bp = firstbuf; bp != 0; bp = bp->next)
    {
        int k = bp->n;
        (void)k;
    }
    return k;
}
