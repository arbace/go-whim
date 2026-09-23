155 declares nothing: the eleven calls whose arguments both have effects
evaluate them in the order gcc already did, now written as a local before the
call; the check measures gcc's order in both binaries' code.
