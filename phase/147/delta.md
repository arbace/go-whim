147 declares nothing: SIGHUP and SIGTERM end the editor as they did, at the
wait in ui_inchar() -- the only place the core unblocks them -- and no longer
inside the handler.  The libc surface grows by pipe2, which the check requires
exactly.  Its pty probes send both signals while waiting, and SIGTERM while
busy, which interrupts as before.
