/* What whim-vim.c's host region needs of macOS that macOS spells otherwise,
 * included ahead of the file (clang -include guest/mac/shim.h): the C
 * reference built on the Mac for the suite (doc/GUEST.md, *Running on the
 * Mac*).  whim-vim.c is produced, not edited, and its host is written for
 * gcc on Linux in three places: XTABS, which macOS calls OXTABS; pipe2, which
 * macOS lacks -- pipe and the two flags after it, before anything forks; and
 * __builtin_setjmp and __builtin_longjmp, which clang does not offer on arm64
 * ("not supported for the current target") -- _setjmp and _longjmp on a
 * jmp_buf of the shim's, the host's one jump being from host_exit back to
 * main.  And gettimeofday, found on the Mac itself (Homebrew clang 23.1.1,
 * arm64-apple-darwin23.6.0): glibc and musl declare it through the headers
 * the host includes, macOS only in <sys/time.h> -- declared here as macOS
 * does, with no header, since a header included this early would define the
 * names the core declares as its own (INT_MAX, PATH_MAX: phase 43).  On
 * Linux the shim changes nothing the host does (XTABS is defined; the
 * pipe2, the jump and the declaration are this one's, to the same effect). */
#include <setjmp.h>
#include <fcntl.h>
#include <termios.h>
#include <unistd.h>

#ifndef XTABS
#define XTABS OXTABS
#endif

static int whim_pipe2(int fd[2], int flags)
{
    if (pipe(fd) != 0)
        return -1;
    for (int i = 0; i < 2; i++)
    {
        if ((flags & O_CLOEXEC) && fcntl(fd[i], F_SETFD, FD_CLOEXEC) == -1)
            goto fail;
        if ((flags & O_NONBLOCK) && fcntl(fd[i], F_SETFL, fcntl(fd[i], F_GETFL) | O_NONBLOCK) == -1)
            goto fail;
    }
    return 0;
fail:
    close(fd[0]);
    close(fd[1]);
    return -1;
}
#define pipe2 whim_pipe2

struct timeval;
int gettimeofday(struct timeval *__restrict, void *__restrict);

static jmp_buf whim_jump;
#define __builtin_setjmp(buf) _setjmp(whim_jump)
#define __builtin_longjmp(buf, v) _longjmp(whim_jump, (v))
