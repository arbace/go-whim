;; bin/whimsical, the launcher: the editor ((whimsical host)'s run) on the
;; terminal host ((whimsical term)), with the command line's arguments, the
;; process ending with the editor's status -- whim-vim.c's main, whose
;; __builtin_setjmp is run's catch of host_exit.  The last form of the boot
;; file: it sets what Chez starts with.
;;
;; Output is not buffered by Scheme: the host writes with write(2) on fd 1
;; and 2 directly, and nothing here prints through a port.
(import (chezscheme) (whimsical host) (whimsical term))

(suppress-greeting #t)
(scheme-start
  (lambda args
    (exit (run (make-term-host) args))))
