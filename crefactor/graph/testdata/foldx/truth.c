static int p_tbs;

int p_ic;

int use(int);

struct s
{
    int ic;
    int headlen;
};

struct s *orgpat;

    int
main(void)
{
    int noic = use(0);
    int findall = use(1);
    int x;
    p_ic = use(2);
    orgpat->ic = ((p_ic || !noic) && 1);
    x = ((p_ic || !noic) && 1);
    x = ((p_ic || !noic) && 1);
    x = ((p_ic || !noic) && 1);
    x = (p_ic && 1);
    x = (p_ic == 1);
    x = (p_ic + 1) * 2;
    return x;
}
