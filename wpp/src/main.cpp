// bin/whimpp, the launcher: the editor (whimpp::run) on the terminal host
// (whimpp::new_term), with the command line's arguments, the process
// ending with the editor's status -- whim-vim.c's main, whose
// __builtin_setjmp is run's catch of host_exit.  Output is not buffered by
// C++: the host writes with write(2) on fd 1 and 2 directly.

#include "host.hpp"
#include "term.hpp"

#include <string>
#include <vector>

int main(int argc, char **argv)
{
    std::vector<std::string> args(argv, argv + argc);
    return whimpp::run(whimpp::new_term(), args);
}
