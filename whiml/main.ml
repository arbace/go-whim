(* bin/whiml, the launcher: the editor (Host.run) on the terminal host
   (Term), with the command line's arguments, the process ending with the
   editor's status -- whim-vim.c's main, whose __builtin_setjmp is run's
   catch of host_exit. *)

let () = exit (Host.run (Term.make ()) (Array.to_list Sys.argv))
