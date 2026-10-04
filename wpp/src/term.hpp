// The terminal host: a Host on the process's own file descriptors 0, 1 and
// 2 (term.cpp).

#pragma once

#include "host.hpp"

#include <memory>

namespace whimpp {

// A terminal host, not yet catching signals: the editor's init starts it.
std::unique_ptr<Host> new_term();

} // namespace whimpp
