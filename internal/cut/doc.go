// Package cut holds the phase cutters: the tools a phase program calls to
// remove one named thing from the source.
//
// They share a rule that is GOALS.md's first: cut the entry point and let
// the compiler find the rest.  A cutter does not decide what becomes
// unreachable next -- that is the sweep's job -- and it REFUSES on a name it
// cannot find, because a silent miss leaves the thing in place and the report
// would say the work was done.
package cut
