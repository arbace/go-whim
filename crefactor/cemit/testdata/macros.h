/* Macros for the tests: Canonical refuses a #define in the text it prints,
   so a test that needs one includes it from here. */
struct ts { long tv_sec; long tv_nsec; };
struct st { struct ts st_mtim; };
#define st_mtime st_mtim.tv_sec
#define DECLARE(n) int n;
