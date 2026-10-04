(* Editors are instances: four run at once in one process, each on a domain
   and a host of this program's own -- keys from a string, the screen into a
   buffer, a fixed size -- and none sees another's text.
   editor/host_test.go's TestEditorsAreInstances, in OCaml; whiml_test.go
   compiles it against the modules `make bin/whiml` compiled. *)

(* A host with no operating system under it: what an embedding program
   would write. *)
let fake_host keys screen messages : Host.t =
  let at = ref 0 in
  {
    init = (fun _ -> ());
    win_size = (fun () -> Some (24, 80));
    term_start = (fun () -> ());
    term_stop = (fun () -> ());
    tty_keys = (fun _ -> Some (0x7f, 3, true, true));
    now_ms = (fun () -> 0);
    time = (fun () -> 0);
    delay = (fun _ _ -> ());
    wait_for_input = (fun _ -> !at < String.length keys);
    read_input =
      (fun buf off count ->
        let n = min count (String.length keys - !at) in
        Bytes.blit_string keys !at buf off n;
        at := !at + n;
        n);
    raise = (fun _ -> ());
    suspend = (fun () -> ());
    message = (fun buf off n _ -> Buffer.add_subbytes messages buf off n);
    write =
      (fun buf off n ->
        Buffer.add_subbytes screen buf off n;
        n);
  }

let text i = Printf.sprintf "editing number %d" i

let contains hay needle =
  let n = String.length needle and m = String.length hay in
  let rec go i = i + n <= m && (String.sub hay i n = needle || go (i + 1)) in
  go 0

let n = 4

let () =
  let runs =
    List.init n (fun i ->
        let screen = Buffer.create 4096 and messages = Buffer.create 256 in
        let d = Domain.spawn (fun () -> Host.run (fake_host ("i" ^ text i ^ "\027:q!\r") screen messages) [ "whiml" ]) in
        (screen, messages, d))
  in
  let bad = ref false in
  List.iteri
    (fun i (screen, messages, d) ->
      let status = Domain.join d in
      if status <> 0 then begin
        Printf.printf "editor %d: status %d; messages %S\n" i status (Buffer.contents messages);
        bad := true
      end;
      let s = Buffer.contents screen in
      for j = 0 to n - 1 do
        if contains s (text j) <> (i = j) then begin
          Printf.printf "editor %d's screen shows editor %d's text: %b\n" i j (contains s (text j));
          bad := true
        end
      done)
    runs;
  if !bad then exit 1 else Printf.printf "ok: %d editors, each its own\n" n
