(* The host: what the editor core asks of the world it runs in, behind a
   record of functions.  In whim-vim.c the host is everything from the first
   #include to the end of the file, and the core calls it by name --
   host_write, musl_read_input and the rest.  Here the core (the module
   Editor, generated) still calls those names, and each is a field of the
   record Editor.glue the editor carries, the C's signature on the core's
   side, glue to the host the editor runs on: editor/host.go's Host,
   caprice's Caprice.Host, whimsy's host::Host, whimsical's host record.
   What is the editor's and not the host's -- the arena, which the C host
   keeps as the core's memory -- is the editor's (Rt).  So a process holds
   any number of editors, each on its own host and domain: the terminal
   (Term) or anything else. *)

(* What the core needs of the world it runs in: a terminal, a clock, input
   with a timeout, the signals, output.  A buffer is a Bytes, a start and a
   count, as Unix's read and write take them. *)
type t = {
  init : (int -> unit) -> unit;
      (** start catching the signals the editor handles; the host calls the
          function (the core's deathtrap) for a SIGHUP or a SIGTERM, on the
          editor's domain, where the C's handler would have run *)
  win_size : unit -> (int * int) option;  (** the terminal's rows and columns *)
  term_start : unit -> unit;  (** raw mode *)
  term_stop : unit -> unit;  (** and out of it *)
  tty_keys : int -> (int * int * bool * bool) option;
      (** fd's erase and interrupt characters, and whether it maps CR to NL
          on input and NL to CR NL on output; None when fd is no terminal *)
  now_ms : unit -> int;  (** milliseconds of a clock that starts at the first call *)
  time : unit -> int;  (** the Unix time *)
  delay : int -> bool -> unit;  (** sleep ms; interruptible lets the terminal relax meanwhile *)
  wait_for_input : int -> bool;
      (** whether input -- or a signal the core reads as input -- is there
          within ms milliseconds (for ever when negative) *)
  read_input : Bytes.t -> int -> int -> int;
      (** input read into the buffer: the count, 0 at its end, -1 when a
          signal came first; a signal the core reads as input is written as
          the keys that stand for it *)
  raise : int -> unit;  (** the process signalled *)
  suspend : unit -> unit;  (** and stopped *)
  message : Bytes.t -> int -> int -> bool -> unit;
      (** a message outside the screen, to the error stream when true *)
  write : Bytes.t -> int -> int -> int;  (** the screen's output: the count written, or -1 *)
}

(* The C host's arena: 1 GiB of the editor's memory, never freed; its
   exhaustion ends the editor, as the C's ends the process. *)
let host_alloc ed n =
  match Rt.arena_alloc ed n with
  | Some p, _ -> p
  | None, used ->
      let m = Printf.sprintf "whim-vim: host arena exhausted: %d bytes, %d used, request %d\n" Rt.arena_bytes used n in
      (* the message from a frame of the editor's stack, as the C's from its own *)
      let len = String.length m in
      let fr = Rt.frame_push ed (Rt.align16 len) in
      Rt.mem_image ed fr m;
      ed.Rt.glue.Editor.host_message ed fr len 1;
      raise (Rt.Exit 1)

(* The C host's 17 functions, as the core calls them: each a function of the
   editor and the C's arguments, glue to the host h. *)
let glue (h : t) : Editor.glue =
  {
    musl_host_init = (fun ed -> h.init (fun signal -> Editor.deathtrap ed signal));
    musl_get_winsize =
      (fun ed rows cols ->
        match h.win_size () with
        | Some (r, c) ->
            Rt.st_s32 ed rows r;
            Rt.st_s32 ed cols c;
            1 (* OK *)
        | None -> 0 (* FAIL *));
    musl_term_start = (fun _ -> h.term_start ());
    musl_term_stop = (fun _ -> h.term_stop ());
    musl_tty_keys =
      (fun ed fd bs intr cr nlcr ->
        match h.tty_keys fd with
        | Some (e, i, icrnl, onlcr) ->
            Rt.st_s32 ed bs e;
            Rt.st_s32 ed intr i;
            Rt.st_s32 ed cr (Bool.to_int icrnl);
            Rt.st_s32 ed nlcr (Bool.to_int onlcr);
            1
        | None -> 0);
    musl_now_ms = (fun _ -> h.now_ms ());
    host_time = (fun _ -> h.time ());
    musl_delay = (fun _ ms interruptible -> h.delay ms (interruptible <> 0));
    musl_wait_for_input = (fun _ ms -> Bool.to_int (h.wait_for_input ms));
    (* no room for a negative length, and -1 for it, as read(2) of
       (size_t)len answers; the host still takes the signals it reads as
       input, as the C does before its read *)
    musl_read_input =
      (fun ed buf len ->
        let n = h.read_input ed.Rt.mem buf (max len 0) in
        if len < 0 then -1 else n);
    host_raise = (fun _ signal -> h.raise signal);
    musl_suspend = (fun _ -> h.suspend ());
    host_exit = (fun _ r -> raise (Rt.Exit r));
    host_message =
      (fun ed msg len err ->
        let n = if len < 0 then Bytes.index_from ed.Rt.mem msg '\000' - msg else len in
        h.message ed.Rt.mem msg n (err <> 0));
    host_write = (fun ed s len -> if len < 0 then -1 else if len = 0 then 0 else h.write ed.Rt.mem s len);
    host_alloc;
    vim_snprintf = Snprintf.vim_snprintf;
  }

(* An editor on h, run with args -- the C's argv, the program's name first --
   to its end: the status host_exit gave, or vim_main's.  Each run is an
   editor of its own, its memory and its arena, so a process may run several
   at once, each on its domain. *)
let run h args =
  let ed = Editor.new_editor (glue h) in
  let argc = List.length args in
  let size = List.fold_left (fun n a -> n + String.length a + 1) (8 * (argc + 1)) args in
  (* argv and its strings at the bottom of the editor's stack, where the C's
     are above main's frame *)
  let argv = Rt.frame_push ed (Rt.align16 size) in
  let _ =
    List.fold_left
      (fun (i, at) a ->
        Rt.st_ptr ed (argv + (8 * i)) at;
        Rt.mem_image ed at a;
        (i + 1, at + String.length a + 1))
      (0, argv + (8 * (argc + 1)))
      args
  in
  try Editor.vim_main ed argc argv with Rt.Exit code -> code
