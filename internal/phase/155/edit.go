package p155

// Whim phase 155 -- call arguments with effects are evaluated in gcc's order.  See GOAL.md.
//
// gcc evaluates call arguments right to left; Go, left to right.  Where two
// arguments both have effects (internal/ccx), the one gcc evaluates first
// becomes a local computed before the call -- eleven calls.
//
// THE INPUT BINARY IS BUILT before the edit, by the plan (internal/build's
// OldBinary), from the boundary's own makefile flags, as $state/old beside
// $state/old.c, for the check.
