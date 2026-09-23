150 declares nothing: the regexp engine's stack holds the same records and the
same star and look-behind data in three typed stacks instead of one byte
array, and counts the same bytes, so 'maxmempattern' fails at the same value
-- the check finds that value by bisection and requires it of both binaries.
