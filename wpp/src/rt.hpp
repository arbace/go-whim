// The runtime the generated C++ is written against (doc/CPP.md): the
// parallel chunks of match_lines.  It includes nothing of the standard
// library's, so that no macro of a C header reaches the generated header's
// names.

#pragma once

namespace whimpp {

// chunks_run runs work(ctx, from, to) over [0, n) in chunks, on as many
// threads as the machine has cores, and says whether every chunk's work
// did (rt.cpp).
bool chunks_run(long n, bool (*work)(void *ctx, long from, long to), void *ctx);

// chunks is chunks_run of a callable: the parallel body of match_lines,
// which the C writes as one loop over the range (editor/chunks.go is the
// Go's).
template <class F>
bool chunks(long n, F work)
{
    return chunks_run(n, [](void *ctx, long from, long to) { return (*static_cast<F *>(ctx))(from, to); }, &work);
}

} // namespace whimpp
