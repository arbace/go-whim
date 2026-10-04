(* The runtime the generated editor is written against (doc/OCAML.md): C's
   memory, kept as C keeps it.  Every C object of an editor lives in one
   Bytes, laid out as the C lays it out on amd64 -- below 64 KiB the null
   page and the function pointers, then the file-scope objects and the
   string literals, then the main thread's stack of call frames, the arena
   the host allocates from, and the stacks of the domains the parallel :%s
   runs on -- and a pointer is an int offset into it.  So the translation
   reads and writes bytes at offsets, and C's pointer arithmetic,
   comparison and punning are the machine's.

   C's integers are OCaml's int, 63 bits: the 8-, 16- and 32-bit types
   wrapped where C wraps them; long and unsigned long the int itself, an
   unsigned long its 64 bits read as a signed long -- exact for every value
   under 2^62 in magnitude, and for the low 63 bits of a sum, a product or
   a left shift past it -- ordered, divided and shifted right as unsigned
   by the functions here.  A C bool is OCaml's bool, a byte 0 or 1 in
   memory. *)

(* host_exit: what run catches, the C's longjmp to main *)
exception Exit of int

(* What an editor's domains share: the arena's bump pointer and end, how far
   it is zeroed, the lock they take, and the stacks the domains of the
   parallel loop run on. *)
type shared = {
  arena_base : int;
  mutable next : int;
  arena_end : int;
  mutable zeroed : int;
  lock : Mutex.t;
  mutable stacks : int list;
  mutable stack_next : int;
  stack_end : int;
}

(* An editor: its memory, the stack its domain's frames are on, the host's
   functions (the record the core's glue type says), and what its domains
   share.  fork makes the record a domain of the parallel :%s runs with: the
   same memory, a stack of its own. *)
type 'g ed = {
  mem : Bytes.t;
  mutable sp : int;
  limit : int;
  glue : 'g;
  shared : shared;
}

let stack_bytes = 8 * 1024 * 1024 (* the main domain's frames *)
let thread_stack_bytes = 1024 * 1024 (* each worker's *)
let thread_stacks = 128
let arena_bytes = 1024 * 1024 * 1024 (* the C host's HOST_ARENA_BYTES *)
let align16 n = (n + 15) land lnot 15

(* n bytes at p set to zero *)
let zero_range mem p q = if q > p then Bytes.fill mem p (q - p) '\000'

(* An editor whose memory holds data_end bytes of the core's (the null page,
   the segment and the literals, which the core then fills in), on the
   host's functions glue.  The Bytes is made unfilled: its pages are the
   kernel's zeros until touched, and what the editor is handed -- the data,
   a frame, the arena as it grows -- is zeroed first. *)
let make_editor glue data_end =
  let stack = align16 data_end in
  let arena = stack + stack_bytes in
  let stacks = arena + arena_bytes in
  let size = stacks + (thread_stacks * thread_stack_bytes) in
  let mem = Bytes.create size in
  zero_range mem 0 stack;
  {
    mem;
    sp = stack;
    limit = stack + stack_bytes;
    glue;
    shared =
      {
        arena_base = arena;
        next = arena;
        arena_end = arena + arena_bytes;
        zeroed = arena;
        lock = Mutex.create ();
        stacks = [];
        stack_next = stacks;
        stack_end = size;
      };
  }

(* A record of ed for another domain: its own stack. *)
let fork ed stack = { ed with sp = stack; limit = stack + thread_stack_bytes }

(* n bytes of the arena, 16-aligned as max_align_t is, or None when it has
   not that many (the host says so and exits, as the C's does); and how much
   is used.  Several domains may allocate at once: the regex engines of the
   parallel :%s. *)
let arena_alloc ed n =
  let sh = ed.shared in
  let want = (n + 15) land lnot 15 in
  Mutex.protect sh.lock (fun () ->
      let p = sh.next in
      if want < n || n < 0 || want > sh.arena_end - p then (None, p - sh.arena_base)
      else begin
        let next = p + want in
        if next > sh.zeroed then begin
          (* zero ahead, a MiB at a time *)
          let z = min sh.arena_end (next + (1024 * 1024)) in
          zero_range ed.mem sh.zeroed z;
          sh.zeroed <- z
        end;
        sh.next <- next;
        (Some p, p - sh.arena_base)
      end)

(* --- memory ---------------------------------------------------------------- *)

external get16u : Bytes.t -> int -> int = "%caml_bytes_get16u"
external get32u : Bytes.t -> int -> int32 = "%caml_bytes_get32u"
external get64u : Bytes.t -> int -> int64 = "%caml_bytes_get64u"
external set16u : Bytes.t -> int -> int -> unit = "%caml_bytes_set16u"
external set32u : Bytes.t -> int -> int32 -> unit = "%caml_bytes_set32u"
external set64u : Bytes.t -> int -> int64 -> unit = "%caml_bytes_set64u"

let ld_u8 ed p = Char.code (Bytes.unsafe_get ed.mem p)
let ld_s8 ed p = (Char.code (Bytes.unsafe_get ed.mem p) lsl 55) asr 55
let ld_u16 ed p = get16u ed.mem p
let ld_s16 ed p = (get16u ed.mem p lsl 47) asr 47
let ld_s32 ed p = Int32.to_int (get32u ed.mem p)
let ld_u32 ed p = Int32.to_int (get32u ed.mem p) land 0xffffffff
let ld_s64 ed p = Int64.to_int (get64u ed.mem p)
let ld_u64 = ld_s64
let ld_ptr = ld_s64
let ld_bool ed p = Bytes.unsafe_get ed.mem p <> '\000'
let st_u8 ed p v = Bytes.unsafe_set ed.mem p (Char.unsafe_chr (v land 0xff))
let st_s8 = st_u8
let st_u16 ed p v = set16u ed.mem p (v land 0xffff)
let st_s16 = st_u16
let st_s32 ed p v = set32u ed.mem p (Int32.of_int v)
let st_u32 = st_s32
let st_s64 ed p v = set64u ed.mem p (Int64.of_int v)
let st_u64 = st_s64
let st_ptr = st_s64
let st_bool ed p v = Bytes.unsafe_set ed.mem p (if v then '\001' else '\000')

(* n bytes from src to dst, overlapping or not: a struct's copy, memmove *)
let mem_copy ed dst src n = if n > 0 then Bytes.blit ed.mem src ed.mem dst n
let mem_zero ed p n = if n > 0 then Bytes.fill ed.mem p n '\000'
let mem_fill ed p c n = if n > 0 then Bytes.fill ed.mem p n (Char.unsafe_chr (c land 0xff))

(* The bytes of s written at p: the image of the core's initial data. *)
let mem_image ed p s = Bytes.blit_string s 0 ed.mem p (String.length s)

(* The C string at p, as an OCaml string. *)
let mem_string ed p =
  let q = Bytes.index_from ed.mem p '\000' in
  Bytes.sub_string ed.mem p (q - p)

(* --- a call's frame -------------------------------------------------------- *)

(* The locals of a call that live in memory: n bytes (a multiple of 16) of
   the domain's stack, zeroed; frame_pop gives them back where the call
   returns. *)
let frame_push ed n =
  let fr = ed.sp in
  let sp = fr + n in
  if sp > ed.limit then failwith "whiml: the stack of call frames overflowed";
  ed.sp <- sp;
  Bytes.fill ed.mem fr n '\000';
  fr

let frame_pop ed fr = ed.sp <- fr

(* --- C's integers ---------------------------------------------------------- *)

(* an int converted to a C type: its bits, sign- or zero-extended *)
let to_i8 x = (x lsl 55) asr 55
let to_u8 x = x land 0xff
let to_i16 x = (x lsl 47) asr 47
let to_u16 x = x land 0xffff
let to_i32 x = (x lsl 31) asr 31
let to_u32 x = x land 0xffffffff

(* int: a left shift wraps *)
let i32_shl a n = to_i32 (a lsl n)

(* unsigned int: wrapped to 32 bits *)
let u32_add a b = (a + b) land 0xffffffff
let u32_sub a b = (a - b) land 0xffffffff
let u32_mul a b = (a * b) land 0xffffffff
let u32_shl a n = (a lsl n) land 0xffffffff
let u32_not a = a lxor 0xffffffff

(* unsigned long: its bits as a signed long, so ordered, divided and shifted
   right as unsigned (+, -, * and lsl are OCaml's own: their low bits are
   the C's) *)
let u64_lt a b = a lxor min_int < b lxor min_int
let u64_le a b = a lxor min_int <= b lxor min_int
let u64_gt a b = a lxor min_int > b lxor min_int
let u64_ge a b = a lxor min_int >= b lxor min_int
let u64_div a b = if a >= 0 && b > 0 then a / b else Int64.to_int (Int64.unsigned_div (Int64.of_int a) (Int64.of_int b))
let u64_rem a b = if a >= 0 && b > 0 then a mod b else Int64.to_int (Int64.unsigned_rem (Int64.of_int a) (Int64.of_int b))
let u64_shr a n = if a >= 0 then a lsr n else Int64.to_int (Int64.shift_right_logical (Int64.of_int a) n)

(* --- function pointers ------------------------------------------------------- *)

(* A function's address: its index in the core's tables, in the null page,
   where no object is. *)
let fn_ptr i = (i + 1) * 16
let fn_index p = (p lsr 4) - 1

(* --- a loop over lines, in parallel ---------------------------------------------- *)

(* the workers the parallel loop runs on: the processors the machine has,
   at most 16 -- or WHIML_WORKERS, when the environment says how many (1:
   the C's loop, on the caller's domain).  A domain is dear in OCaml: each
   one alive is stopped by every minor collection of the others, and the
   heavy case ran 0.30-0.32 s on 8 or 16 workers against 0.40-0.46 on 64
   (doc/OCAML.md). *)
let workers =
  lazy
    (let n =
       match Sys.getenv_opt "WHIML_WORKERS" with
       | Some s -> ( try int_of_string s with _ -> 1)
       | None -> min 16 (Domain.recommended_domain_count ())
     in
     min thread_stacks (max 1 n))

(* a worker's stack: one given back, or a new one of the region's *)
let take_stack ed =
  let sh = ed.shared in
  Mutex.protect sh.lock (fun () ->
      match sh.stacks with
      | s :: rest ->
          sh.stacks <- rest;
          s
      | [] ->
          let s = sh.stack_next in
          if s + thread_stack_bytes > sh.stack_end then failwith "whiml: no stack left for another worker";
          sh.stack_next <- s + thread_stack_bytes;
          s)

let give_stack ed s =
  let sh = ed.shared in
  Mutex.protect sh.lock (fun () -> sh.stacks <- s :: sh.stacks)

(* work over [0, n) in chunks at once, and whether every chunk's work did:
   the parallel body of a function the C writes as one loop over a range
   (match_lines, as editor/chunks.go's Chunks).  About four chunks a
   worker, none smaller than 64, on the workers (above), each a domain
   with a stack of its own, the caller's among them; a range of one chunk, or one worker, runs on the caller's domain,
   and a chunk that did not stops the chunks not yet started.  (work ed from
   to) must be the loop's over its part and write nothing another part
   reads.

   The domains are made for the loop and joined after it: OCaml's minor
   collections stop every domain alive, and a pool of them kept waiting
   made the heavy case four times slower (doc/OCAML.md). *)
let chunks ed n work =
  let workers = Lazy.force workers in
  let w = 4 * workers in
  let size = max 64 ((n + w - 1) / w) in
  if workers = 1 || size >= n then work ed 0 n
  else begin
    let count = (n + size - 1) / size in
    let next = Atomic.make 0 in
    let failed = Atomic.make false in
    let error = Atomic.make None in
    let run ted =
      let rec loop () =
        if not (Atomic.get failed || Atomic.get error <> None) then begin
          let i = Atomic.fetch_and_add next 1 in
          if i < count then begin
            let from = i * size in
            (try if not (work ted from (min n (from + size))) then Atomic.set failed true
             with e -> ignore (Atomic.compare_and_set error None (Some e)));
            loop ()
          end
        end
      in
      loop ()
    in
    let stacks = List.init (min workers count) (fun _ -> take_stack ed) in
    let domains = List.map (fun s -> Domain.spawn (fun () -> run (fork ed s))) (List.tl stacks) in
    run (fork ed (List.hd stacks));
    List.iter Domain.join domains;
    List.iter (give_stack ed) stacks;
    (match Atomic.get error with Some e -> raise e | None -> ());
    not (Atomic.get failed)
  end
