(* The terminal host: a Host.t on the process's own file descriptors 0, 1 and
   2 -- the OCaml port of the half of whim-vim.c's host that asks the
   operating system for something, from host_winch_pending to host_write,
   function by function, as whimsy's term.rs ports it.  It is the host
   bin/whiml runs the editor with.

   Unix has most of what it needs; what it has not -- the signal handlers,
   real ones installed with sigaction(2) as the C installs them, the raw
   mode's IEXTEN, ONLCR and XTABS, the window's size -- is the C host's own,
   in term_stubs.c.  The handlers store to C statics and the SIGHUP/SIGTERM
   one writes a byte to the pipe this host made, exactly as the C's do; so
   one process has one terminal, as in the C.

   Deviations from the C, as whimsy's: host_time's atol spelled out (the
   same digits read); the report a SIGWINCH is read as formatted by
   Printf.sprintf where the C calls vim_snprintf (the same digits). *)

external host_init : Unix.file_descr -> unit = "whiml_host_init"
external pending : int -> bool -> int = "whiml_pending"
external tty_set : bool -> bool -> unit = "whiml_tty_set"
external win_size_raw : unit -> int = "whiml_win_size"
external tty_keys_raw : int -> int = "whiml_tty_keys"
external suspend : unit -> unit = "whiml_suspend"

let winch, tstp, int, death = (0, 1, 2, 3)
let peek f = pending f true <> 0
let take f = pending f false

(* musl_get_winsize's ioctl *)
let win_size () =
  let v = win_size_raw () in
  if v < 0 then None else Some (v lsr 16, v land 0xffff)

(* musl_tty_keys's tcgetattr *)
let tty_keys fd =
  let v = tty_keys_raw fd in
  if v < 0 then None else Some (v land 0xff, (v lsr 8) land 0xff, v land 0x10000 <> 0, v land 0x20000 <> 0)

(* host_time: WHIM_TIME, when the environment holds it (phase 99), as atol
   reads it; the clock's otherwise *)
let atol s =
  let n = String.length s in
  let rec skip i = if i < n && (s.[i] = ' ' || (s.[i] >= '\t' && s.[i] <= '\r')) then skip (i + 1) else i in
  let i = skip 0 in
  let neg = i < n && s.[i] = '-' in
  let i = if i < n && (s.[i] = '-' || s.[i] = '+') then i + 1 else i in
  let rec digits i v = if i < n && s.[i] >= '0' && s.[i] <= '9' then digits (i + 1) ((v * 10) + Char.code s.[i] - 48) else v in
  let v = digits i 0 in
  if neg then -v else v

let time () =
  match Sys.getenv_opt "WHIM_TIME" with
  | Some pinned when pinned <> "" -> atol pinned
  | _ -> int_of_float (Unix.time ())

(* write(2) once: its count, or -1 *)
let write_once fd buf off n = try Unix.write fd buf off n with Unix.Unix_error _ -> -1

let make () : Host.t =
  let pipe = ref None in
  let raw = ref false in
  let now_base = ref None in
  let deathtrap = ref (fun _ -> ()) in
  (* host_deliver_death *)
  let deliver_death () =
    (match !pipe with
    | Some (r, _) ->
        let b = Bytes.create 16 in
        let rec drain () = match Unix.read r b 0 16 with n when n > 0 -> drain () | _ -> () | exception Unix.Unix_error _ -> () in
        drain ()
    | None -> ());
    let signal = take death in
    if signal <> 0 then !deathtrap signal
  in
  let init dt =
    deathtrap := dt;
    (match !pipe with
    | Some (r, w) ->
        Unix.close r;
        Unix.close w
    | None -> ());
    let r, w = Unix.pipe ~cloexec:true () in
    Unix.set_nonblock r;
    Unix.set_nonblock w;
    pipe := Some (r, w);
    host_init w
  in
  let now_ms () =
    let t = Unix.gettimeofday () in
    let sec = Float.to_int t in
    let base = match !now_base with Some b -> b | None -> now_base := Some sec; sec in
    ((sec - base) * 1000) + (Float.to_int ((t -. Float.of_int sec) *. 1e6) / 1000)
  in
  let delay ms interruptible =
    let relax = interruptible && !raw && ms > 500 in
    if relax then tty_set false true;
    (try if ms > 0 then Unix.sleepf (Float.of_int ms /. 1000.) with Unix.Unix_error _ -> ());
    if relax then tty_set true false
  in
  (* musl_wait_for_input: select(2) on the input and the pipe *)
  let wait_for_input ms =
    let timeout = if ms >= 0 then Float.of_int ms /. 1000. else -1. in
    let rec loop () =
      deliver_death ();
      if peek winch || peek tstp || peek int then true
      else begin
        let fds = match !pipe with Some (r, _) -> [ Unix.stdin; r ] | None -> [ Unix.stdin ] in
        match Unix.select fds [] [] timeout with
        | exception Unix.Unix_error (Unix.EINTR, _, _) -> loop ()
        | exception Unix.Unix_error _ -> false
        | ready, _, _ -> (
            match !pipe with
            | Some (r, _) when List.mem r ready -> loop ()
            | _ -> List.mem Unix.stdin ready)
      end
    in
    loop ()
  in
  let read_input buf off len =
    deliver_death ();
    let put s =
      Bytes.blit_string s 0 buf off (String.length s);
      String.length s
    in
    let from_int = if take int <> 0 && len >= 1 then Some (put "\003") else None in
    match from_int with
    | Some n -> n
    | None -> (
        let from_winch =
          if take winch <> 0 then
            match win_size () with
            | Some (rows, cols) when len >= 32 -> Some (put (Printf.sprintf "\027[48;%d;%d;0;0t" rows cols))
            | _ -> None
          else None
        in
        match from_winch with
        | Some n -> n
        | None -> (
            if take tstp <> 0 && len >= 5 then put "\027[?1z"
            else try Unix.read Unix.stdin buf off len with Unix.Unix_error _ -> -1))
  in
  let message buf off n err =
    let fd = if err then Unix.stderr else Unix.stdout in
    let rec go o = if o < n then match write_once fd buf (off + o) (n - o) with w when w > 0 -> go (o + w) | _ -> () in
    go 0
  in
  {
    init;
    win_size;
    term_start = (fun () -> raw := true; tty_set true false);
    term_stop = (fun () -> raw := false; tty_set false false);
    tty_keys;
    now_ms;
    time;
    delay;
    wait_for_input;
    read_input;
    raise = (fun signal -> Unix.kill (Unix.getpid ()) signal);
    suspend;
    message;
    write = (fun buf off n -> write_once Unix.stdout buf off n);
  }
