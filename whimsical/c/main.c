/*
 * bin/whimsical's main: Chez Scheme's kernel (libkernel.a) started on two
 * boot files linked into the executable -- petite.boot, the run-time system
 * without the compiler, and whimsical.boot, the editor's libraries and the
 * launcher (main.ss) -- each converted to vfasl, which loads without
 * relocation (doc/SCHEME.md, §8.2).  boot.s holds their bytes.
 *
 * Sscheme_start hands the scheme-start procedure argv[1..]; the editor wants
 * the C's argv, its program's name first, so argv[0] is passed twice.
 */
#include <stdlib.h>
#include "scheme.h"

extern char whimsical_petite[], whimsical_petite_end[];
extern char whimsical_app[], whimsical_app_end[];

int main(int argc, const char *argv[])
{
    const char **args = malloc(sizeof(char *) * (size_t)(argc + 2));
    int i, r;
    args[0] = argv[0];
    for (i = 0; i < argc; i++)
        args[i + 1] = argv[i];
    args[argc + 1] = NULL;
    Sscheme_init(NULL);
    Sregister_boot_file_bytes("petite", whimsical_petite, whimsical_petite_end - whimsical_petite);
    Sregister_boot_file_bytes("whimsical", whimsical_app, whimsical_app_end - whimsical_app);
    Sbuild_heap(NULL, NULL);
    r = Sscheme_start(argc + 1, args);
    Sscheme_deinit();
    return r;
}
