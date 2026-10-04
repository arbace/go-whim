// The runtime's parallel chunks (rt.hpp): std::jthread, one a core.

#include "rt.hpp"

#include <algorithm>
#include <atomic>
#include <thread>
#include <vector>

namespace whimpp {

namespace {

// The fewest lines a chunk is given: below it a thread costs more than it
// saves.
constexpr long chunk_least = 64;

} // namespace

// chunks_run runs work over [0, n) in chunks, about four a core so that one
// slow chunk does not hold the rest, none smaller than chunk_least; a range
// of one chunk runs on the caller's thread, and a chunk that did not stops
// the chunks not yet started.  The threads are joined as the vector of
// jthreads goes out of scope.
bool chunks_run(long n, bool (*work)(void *ctx, long from, long to), void *ctx)
{
    n = std::max(n, 0L);
    const long cores = std::max(1L, static_cast<long>(std::thread::hardware_concurrency()));
    const long size = std::max(chunk_least, (n + 4 * cores - 1) / (4 * cores));
    if (size >= n)
    {
        return work(ctx, 0, n);
    }
    std::atomic<bool> failed{false};
    std::atomic<long> next{0};
    {
        std::vector<std::jthread> threads;
        const long nthreads = std::min(cores, (n + size - 1) / size);
        threads.reserve(static_cast<std::size_t>(nthreads));
        for (long i = 0; i < nthreads; ++i)
        {
            threads.emplace_back([&] {
                for (;;)
                {
                    const long from = next.fetch_add(size, std::memory_order_relaxed);
                    if (from >= n || failed.load(std::memory_order_relaxed))
                    {
                        break;
                    }
                    if (!work(ctx, from, std::min(from + size, n)))
                    {
                        failed.store(true, std::memory_order_relaxed);
                    }
                }
            });
        }
    }
    return !failed.load(std::memory_order_relaxed);
}

} // namespace whimpp
