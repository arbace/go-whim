(* vim's own printf, vim_snprintf, from the host half of whim-vim.c: the core
   calls it for every message it formats, and it needs nothing of an
   operating system, so it is the library's and not the host's.  A port of
   whimsical's printf.ss (itself a port of the C -- musl_memchr,
   musl_fmtbase, musl_fmtnum, musl_fmtptr, vim_snprintf/vim_vsnprintf/
   vim_vsnprintf_typval, format_typeof, format_typename, adjust_types,
   get_unsigned_int, parse_fmt_types and skip_to_arg) on the C's memory, the
   editor's one Bytes: the format, the strings and the result are bytes at
   offsets into it.

       vim_snprintf ed buf len fmt args   (* the int the C returns *)

   The C's va_list is args, the call's arguments as a list of ints (each as
   the C passed it after the default promotions), and the rest of it still
   to read: va_arg takes the head, va_copy(ap, ap_start) starts again from
   args.  An argument is read at the type the C's va_arg names (to_i32 for
   int, to_u32 for unsigned int, the int itself for long, varnumber_T, the
   unsigned longs and void * -- an unsigned long's bits as a signed long, as
   Rt keeps them -- and a char * is its offset), and every C type here takes
   one place in the list, so skipping an argument of any type is taking the
   head.

   What the C leaves out of reach is left out here too, as in printf.ss:
   get_unsigned_int is only ever called with overflow_err FALSE, so it
   clamps and never reports; the format errors (E1500-E1507) are written
   into IObuff by this function and given to the core's emsg/iemsg, as the C
   does.  The C strings those errors need in the editor's memory are
   written into the editor's arena the first time a call needs one, all of
   them at once, and kept per memory.

   DEVIATION: va_arg past the last argument -- undefined in C -- reads 0
   here (a null pointer for %s, which prints [NULL]), as in whimsy's and
   whimsical's ports.

   DEVIATION (invisible): the positional arguments' types are an array of
   offsets into the format (-1 for the C's nullptr), where the C allocates
   them on its arena. *)

open Rt

let tmp_len = 350
let max_allowed_string_width = 1048576

(* --- bytes --------------------------------------------------------------- *)

let ch ed p = Bytes.unsafe_get ed.mem p
let is_digit c = c >= '0' && c <= '9'
let digit_value c = Char.code c - 48

let c_strlen ed p = Bytes.index_from ed.mem p '\000' - p

(* Where the literal text that starts at p (not a '%', not NUL) ends: the
   next '%' after p, or the NUL. *)
let literal_end ed p =
  let rec go q = match ch ed q with '%' | '\000' -> q | _ -> go (q + 1) in
  go (p + 1)

let rec skip_digits ed p = if is_digit (ch ed p) then skip_digits ed (p + 1) else p

(* musl_memchr(src, c, n), as an offset *)
let musl_memchr ed src c n =
  let rec go s n = if n = 0 then None else if ld_u8 ed s = c then Some s else go (s + 1) (n - 1) in
  go src n

(* --- the conversions of numbers ---------------------------------------------- *)

let musl_fmtbase = function 'o' -> 8 | 'x' | 'X' -> 16 | _ -> 10
let hex_digit d upper = if d < 10 then 48 + d else if upper then 55 + d else 87 + d

(* v, an unsigned long long (its bits in an int64), in base at dest of tmp,
   '-' first when isneg (v then the two's complement of the magnitude); the
   count written, and a NUL after it. *)
let musl_fmtnum tmp dest v base upper isneg =
  let v = if isneg then Int64.neg v else v in
  let b = Int64.of_int base in
  let rec digits v ds =
    let d = Int64.to_int (Int64.unsigned_rem v b) in
    let ds = hex_digit d upper :: ds in
    let v = Int64.unsigned_div v b in
    if v <> 0L then digits v ds else ds
  in
  let ds = digits v [] in
  let start =
    if isneg then begin
      Bytes.set tmp dest '-';
      dest + 1
    end
    else dest
  in
  let i = List.fold_left (fun i d -> Bytes.set tmp i (Char.chr d); i + 1) start ds in
  Bytes.set tmp i '\000';
  i - dest

(* "0x" and v's sixteen hex digits at dest of tmp *)
let musl_fmtptr tmp dest v =
  Bytes.set tmp dest '0';
  Bytes.set tmp (dest + 1) 'x';
  for i = 0 to 15 do
    let d = Int64.to_int (Int64.logand (Int64.shift_right_logical v (60 - (4 * i))) 15L) in
    Bytes.set tmp (dest + 2 + i) (Char.chr (hex_digit d false))
  done;
  Bytes.set tmp (dest + 18) '\000';
  18

(* bn's binary digits at dest of tmp, as the C's b[] copied there *)
let fmt_binary tmp dest bn =
  let rec digits bn ds =
    let ds = (48 + Int64.to_int (Int64.logand bn 1L)) :: ds in
    let bn = Int64.shift_right_logical bn 1 in
    if bn <> 0L then digits bn ds else ds
  in
  List.fold_left (fun i d -> Bytes.set tmp i (Char.chr d); i + 1) dest (digits bn []) - dest

(* --- the C's strings, in the editor's memory --------------------------------------- *)

type text =
  | Unknown
  | Int
  | Long
  | Longlong
  | Uint
  | Ulong
  | Ulonglong
  | Pointer
  | Percent
  | Char
  | String
  | Null
  | E1500
  | E1501
  | E1502
  | E1504
  | E1505
  | E1507

let texts =
  [
    (Unknown, "unknown");
    (Int, "int");
    (Long, "long int");
    (Longlong, "long long int");
    (Uint, "unsigned int");
    (Ulong, "unsigned long int");
    (Ulonglong, "unsigned long long int");
    (Pointer, "pointer");
    (Percent, "percent");
    (Char, "char");
    (String, "string");
    (Null, "[NULL]");
    (E1500, "E1500: Cannot mix positional and non-positional arguments: %s");
    (E1501, "E1501: format argument %d unused in $-style format: %s");
    (E1502, "E1502: Positional argument %d used as field width reused as different type: %s/%s");
    (E1504, "E1504: Positional argument %d type used inconsistently: %s/%s");
    (E1505, "E1505: Invalid format specifier: %s");
    (E1507, "E1507: Internal error: ap_types or ap_types[idx] is NULL: %d: %s");
  ]

(* a memory -> its strings' addresses, held no longer than the memory *)
module Placed = Ephemeron.K1.Make (struct
  type t = Bytes.t

  let equal = ( == )
  let hash _ = 0
end)

let placed : (text * int) list Placed.t = Placed.create 4
let placed_lock = Mutex.create ()

(* The address of the C string t in ed's memory, every one of texts written
   into the arena the first time any is asked for. *)
let c_string ed t =
  let addrs =
    Mutex.protect placed_lock (fun () ->
        match Placed.find_opt placed ed.mem with
        | Some a -> a
        | None ->
            let size = List.fold_left (fun n (_, s) -> n + String.length s + 1) 0 texts in
            let base = match arena_alloc ed size with Some p, _ -> p | None, _ -> failwith "vim_snprintf: the arena has no room for printf's strings" in
            let _, addrs =
              List.fold_left
                (fun (p, acc) (name, s) ->
                  mem_image ed p s;
                  st_u8 ed (p + String.length s) 0;
                  (p + String.length s + 1, (name, p) :: acc))
                (base, []) texts
            in
            Placed.replace placed ed.mem addrs;
            addrs)
  in
  List.assoc t addrs

(* --- the types of the positional arguments ------------------------------------------ *)

(* format_typeof: the type the conversion at t reads *)
let format_typeof ed t =
  let lm, t =
    match ch ed t with
    | ('h' | 'l') as c -> if c = 'l' && ch ed (t + 1) = 'l' then ('L', t + 2) else (c, t + 1)
    | _ -> ('\000', t)
  in
  let spec, lm =
    match ch ed t with
    | 'i' -> ('d', lm)
    | '*' -> ('d', 'h')
    | 'D' -> ('d', 'l')
    | 'U' -> ('u', 'l')
    | 'O' -> ('o', 'l')
    | c -> (c, lm)
  in
  match spec with
  | '%' -> Percent
  | 'c' -> Char
  | 's' | 'S' -> String
  | 'p' -> Pointer
  | 'b' | 'B' -> Ulonglong
  | 'd' -> ( match lm with '\000' | 'h' -> Int | 'l' -> Long | 'L' -> Longlong | _ -> Unknown)
  | 'u' | 'o' | 'x' | 'X' -> ( match lm with '\000' | 'h' -> Uint | 'l' -> Ulong | 'L' -> Ulonglong | _ -> Unknown)
  | _ -> Unknown

(* format_typename, as the address of the name *)
let format_typename ed t = c_string ed (format_typeof ed t)

(* get_unsigned_int with overflow_err FALSE, its only use: the digits at p
   while the value is under the limit, clamped to it, and where they end.
   Its first character is taken as a digit unasked, as the C's is. *)
let get_unsigned_int ed p =
  let rec go uj p =
    if is_digit (ch ed p) && uj < max_allowed_string_width then go ((10 * uj) + digit_value (ch ed p)) (p + 1)
    else (min uj max_allowed_string_width, p)
  in
  go (to_u32 (ld_s8 ed p - 48)) (p + 1)

(* vim_snprintf(IObuff, emsg_iobuff_room(), e, args...);
   emsg(iobuff_or(e)) -- or iemsg, for skip_to_arg's internal error *)
let rec format_error ed e args internal =
  let ef = c_string ed e in
  let room = Editor.emsg_iobuff_room ed in
  ignore (vim_snprintf ed (Editor.iobuff ed) room ef args);
  let s = Editor.iobuff_or ed ef in
  if internal then ignore (Editor.iemsg ed s) else ignore (Editor.emsg ed s)

(* the error a parse of the types gave, after which the C returns *)
and parse_fmt_types ed fmt =
  let exception Fail in
  let types = ref [||] in
  let any_pos = ref false and any_arg = ref false in
  let fail e args =
    format_error ed e args false;
    raise Fail
  in
  let check_pos_arg () = if !any_pos && !any_arg then fail E1500 [ fmt ] in
  (* adjust_types *)
  let adjust arg typ =
    if arg <= 0 then fail E1505 [ typ ];
    if Array.length !types < arg then types := Array.init arg (fun i -> if i < Array.length !types then !types.(i) else -1);
    let prev = !types.(arg - 1) in
    if prev >= 0 then begin
      if ch ed prev = '*' || ch ed typ = '*' then begin
        let pt = if ch ed typ = '*' then prev else typ in
        if not (List.mem (ch ed pt) [ '*'; 'd'; 'i' ]) then fail E1502 [ arg; format_typename ed prev; format_typename ed typ ]
      end
      else if format_typeof ed typ <> format_typeof ed prev then fail E1504 [ arg; format_typename ed typ; format_typename ed prev ]
    end;
    !types.(arg - 1) <- typ
  in
  (* one conversion, p after its '%': where the next text starts *)
  let conversion p =
    let pos_arg = ref (-1) and ptype = ref (skip_digits ed p) and p = ref p in
    if ch ed !ptype = '$' then begin
      if ch ed !p = '0' then fail E1505 [ fmt ];
      let uj, q = get_unsigned_int ed !p in
      pos_arg := uj;
      any_pos := true;
      check_pos_arg ();
      p := q + 1
    end;
    while List.mem (ch ed !p) [ '0'; '-'; '+'; ' '; '#'; '\'' ] do
      incr p
    done;
    (* the width, and the precision: alike *)
    let width_or_precision () =
      let arg = !p in
      if ch ed arg = '*' then begin
        incr p;
        if is_digit (ch ed !p) then begin
          let uj, q = get_unsigned_int ed !p in
          p := q;
          if ch ed !p <> '$' then fail E1505 [ fmt ];
          incr p;
          any_pos := true;
          check_pos_arg ();
          adjust uj arg
        end
        else begin
          any_arg := true;
          check_pos_arg ()
        end
      end
      else if is_digit (ch ed !p) then begin
        let _, q = get_unsigned_int ed !p in
        p := q;
        if ch ed !p = '$' then fail E1505 [ fmt ]
      end
    in
    width_or_precision ();
    if ch ed !p = '.' then begin
      incr p;
      width_or_precision ()
    end;
    if !pos_arg <> -1 then begin
      any_pos := true;
      check_pos_arg ();
      ptype := !p
    end;
    if ch ed !p = 'h' || ch ed !p = 'l' then begin
      let lm = ch ed !p in
      incr p;
      if lm = 'l' && ch ed !p = 'l' then incr p
    end;
    (match ch ed !p with
    | 'i' | '*' | 'd' | 'u' | 'o' | 'D' | 'U' | 'O' | 'x' | 'X' | 'b' | 'B' | 'c' | 's' | 'S' | 'p' ->
        if !pos_arg = -1 then begin
          any_arg := true;
          check_pos_arg ()
        end
        else adjust !pos_arg !ptype
    | _ -> if !pos_arg <> -1 then fail E1500 [ fmt ]);
    if ch ed !p = '\000' then !p else !p + 1
  in
  try
    if fmt <> 0 then begin
      let rec loop p =
        match ch ed p with '\000' -> () | '%' -> loop (conversion (p + 1)) | _ -> loop (literal_end ed p)
      in
      loop fmt
    end;
    Array.iteri (fun i t -> if t < 0 then fail E1501 [ i + 1; fmt ]) !types;
    Some !types
  with Fail -> None

(* vim_snprintf, vim_vsnprintf and vim_vsnprintf_typval in one: fmt
   formatted with args into str, at most str_m bytes of it with the NUL; the
   length the whole result would have, as an int. *)
and vim_snprintf ed str str_m fmt args =
  match parse_fmt_types ed fmt with None -> 0 | Some types -> format_into ed str str_m fmt args types

and format_into ed str str_m fmt args types =
  let tmp = Bytes.make tmp_len '\000' in
  let str_l = ref 0 in
  let ap = ref args in
  let arg_cur = ref 0 and arg_idx = ref 1 in
  (* va_arg: the next argument, 0 past the last (see DEVIATION above) *)
  let va_arg () =
    match !ap with
    | [] -> 0
    | a :: rest ->
        ap := rest;
        a
  in
  (* skip_to_arg: ap at argument arg_idx (from 1), arg_cur the one before
     it, arg_idx then the one after *)
  let skip_to_arg () =
    if !arg_cur + 1 = !arg_idx then begin
      arg_cur := !arg_idx;
      incr arg_idx
    end
    else begin
      let arg_min =
        if !arg_cur >= !arg_idx then begin
          ap := args;
          0
        end
        else !arg_cur
      in
      let rec loop cur =
        arg_cur := cur;
        if cur < !arg_idx - 1 then begin
          let t = if cur < Array.length types then types.(cur) else -1 in
          if t >= 0 then begin
            (match format_typeof ed t with Percent | Unknown -> () | _ -> ignore (va_arg ()));
            loop (cur + 1)
          end
          else format_error ed E1507 [ cur; fmt ] true
        end
        else begin
          arg_cur := cur + 1;
          incr arg_idx
        end
      in
      loop arg_min
    end
  in
  (* skip_to_arg then va_arg, at the C's types *)
  let arg () =
    skip_to_arg ();
    va_arg ()
  in
  let int_arg () = to_i32 (arg ()) in
  (* n bytes of src from off, as much of them as str has room for; and n
     bytes c the same way.  Both count all n. *)
  let put src off n =
    if !str_l < str_m then Bytes.blit src off ed.mem (str + !str_l) (min n (str_m - !str_l));
    str_l := !str_l + n
  in
  let fill c n =
    if !str_l < str_m then mem_fill ed (str + !str_l) (Char.code c) (min n (str_m - !str_l));
    str_l := !str_l + n
  in
  (* one conversion, p after its '%': where the text after it starts *)
  let conversion p =
    let p = ref p in
    let min_field_width = ref 0 and precision = ref 0 in
    let zero_padding = ref false and precision_specified = ref false and justify_left = ref false in
    let alternate_form = ref false and force_sign = ref false and space_for_positive = ref true in
    let length_modifier = ref '\000' in
    let str_arg = ref ed.mem and str_arg_off = ref 0 and str_arg_l = ref 0 in
    let number_of_zeros_to_pad = ref 0 and zero_padding_insertion_ind = ref 0 in
    let pos_arg = ref (-1) in
    (* %N$: the argument's position *)
    if ch ed (skip_digits ed !p) = '$' then begin
      let uj, q = get_unsigned_int ed !p in
      pos_arg := uj;
      p := q + 1
    end;
    let rec flags () =
      let next () =
        incr p;
        flags ()
      in
      match ch ed !p with
      | '0' -> zero_padding := true; next ()
      | '-' -> justify_left := true; next ()
      | '+' -> force_sign := true; space_for_positive := false; next ()
      | ' ' -> force_sign := true; next ()
      | '#' -> alternate_form := true; next ()
      | '\'' -> next ()
      | _ -> ()
    in
    flags ();
    (* the width *)
    if ch ed !p = '*' then begin
      incr p;
      if is_digit (ch ed !p) then begin
        let uj, q = get_unsigned_int ed !p in
        arg_idx := uj;
        p := q + 1
      end;
      let j = min (int_arg ()) max_allowed_string_width in
      if j >= 0 then min_field_width := j
      else begin
        min_field_width := to_i32 (-j);
        justify_left := true
      end
    end
    else if is_digit (ch ed !p) then begin
      let uj, q = get_unsigned_int ed !p in
      min_field_width := uj;
      p := q
    end;
    (* the precision *)
    if ch ed !p = '.' then begin
      incr p;
      precision_specified := true;
      if is_digit (ch ed !p) then begin
        let uj, q = get_unsigned_int ed !p in
        precision := uj;
        p := q
      end
      else if ch ed !p = '*' then begin
        incr p;
        if is_digit (ch ed !p) then begin
          let uj, q = get_unsigned_int ed !p in
          arg_idx := uj;
          p := q + 1
        end;
        let j = min (int_arg ()) max_allowed_string_width in
        if j >= 0 then precision := j
        else begin
          precision_specified := false;
          precision := 0
        end
      end
    end;
    (* the length modifier *)
    if ch ed !p = 'h' || ch ed !p = 'l' then begin
      length_modifier := ch ed !p;
      incr p;
      if !length_modifier = 'l' && ch ed !p = 'l' then begin
        length_modifier := 'L';
        incr p
      end
    end;
    let fmt_spec =
      match ch ed !p with
      | 'i' -> 'd'
      | 'D' -> length_modifier := 'l'; 'd'
      | 'U' -> length_modifier := 'l'; 'u'
      | 'O' -> length_modifier := 'l'; 'o'
      | c -> c
    in
    if !pos_arg <> -1 then arg_idx := !pos_arg;
    (match fmt_spec with
    | '%' | 'c' | 's' | 'S' -> (
        str_arg_l := 1;
        match fmt_spec with
        | '%' -> str_arg_off := !p
        | 'c' ->
            Bytes.set tmp 0 (Char.chr (to_u8 (int_arg ())));
            (* uchar_arg *)
            str_arg := tmp
        | _ ->
            let s = arg () in
            if s = 0 then begin
              str_arg_off := c_string ed Null;
              str_arg_l := 6
            end
            else begin
              str_arg_off := s;
              str_arg_l :=
                if not !precision_specified then c_strlen ed s
                else if !precision = 0 then 0
                else match musl_memchr ed s 0 (min !precision 0x7fffffff) with Some q -> q - s | None -> !precision
            end;
            if fmt_spec = 'S' then begin
              (* the cells, not the bytes, are the precision and the width *)
              let rec loop p1 i =
                let cell = if ld_u8 ed p1 = 0 then None else Some (Editor.utf_ptr2cells ed p1) in
                match cell with
                | Some cell when not (!precision_specified && i + cell > !precision) -> loop (p1 + Editor.utfc_ptr2len ed p1) (i + cell)
                | _ ->
                    str_arg_l := p1 - !str_arg_off;
                    if !min_field_width <> 0 then min_field_width := !min_field_width + (!str_arg_l - i)
              in
              loop !str_arg_off 0
            end)
    | 'd' | 'u' | 'b' | 'B' | 'o' | 'x' | 'X' | 'p' ->
        (* the argument, at its type, its bits in an int64, and its sign (0
           for any unsigned 0) *)
        let signed v = (v, compare v 0L) in
        let unsigned v = (v, if v = 0L then 0 else 1) in
        let v, arg_sign =
          match fmt_spec with
          | 'p' ->
              length_modifier := '\000';
              unsigned (Int64.of_int (arg ()))
          | 'b' | 'B' -> unsigned (Int64.of_int (arg ()))
          | 'd' -> (
              match !length_modifier with
              | '\000' | 'h' -> signed (Int64.of_int (to_i32 (arg ())))
              | _ -> signed (Int64.of_int (arg ())))
          | _ -> (
              match !length_modifier with
              | '\000' | 'h' -> unsigned (Int64.of_int (to_u32 (arg ())))
              | _ -> unsigned (Int64.of_int (arg ())))
        in
        str_arg := tmp;
        if !precision_specified then zero_padding := false;
        if fmt_spec = 'd' then begin
          if !force_sign && arg_sign >= 0 then begin
            Bytes.set tmp !str_arg_l (if !space_for_positive then ' ' else '+');
            incr str_arg_l
          end
        end
        else if !alternate_form then begin
          if arg_sign <> 0 && List.mem fmt_spec [ 'b'; 'B'; 'x'; 'X' ] then begin
            Bytes.set tmp !str_arg_l '0';
            Bytes.set tmp (!str_arg_l + 1) fmt_spec;
            str_arg_l := !str_arg_l + 2
          end
        end;
        zero_padding_insertion_ind := !str_arg_l;
        if not !precision_specified then precision := 1;
        if not (!precision = 0 && arg_sign = 0) then begin
          let n =
            match fmt_spec with
            | 'p' -> musl_fmtptr tmp !str_arg_l v
            | 'b' | 'B' -> fmt_binary tmp !str_arg_l v
            | 'd' ->
                let v = if !length_modifier = 'h' then Int64.of_int (to_i16 (Int64.to_int v)) else v in
                musl_fmtnum tmp !str_arg_l v 10 false (v < 0L)
            | _ ->
                let v = if !length_modifier = 'h' then Int64.of_int (to_u16 (Int64.to_int v)) else v in
                musl_fmtnum tmp !str_arg_l v (musl_fmtbase fmt_spec) (fmt_spec = 'X') false
          in
          str_arg_l := !str_arg_l + n;
          if !zero_padding_insertion_ind < !str_arg_l && Bytes.get tmp !zero_padding_insertion_ind = '-' then incr zero_padding_insertion_ind;
          if
            !zero_padding_insertion_ind + 1 < !str_arg_l
            && Bytes.get tmp !zero_padding_insertion_ind = '0'
            && (Bytes.get tmp (!zero_padding_insertion_ind + 1) = 'x' || Bytes.get tmp (!zero_padding_insertion_ind + 1) = 'X')
          then zero_padding_insertion_ind := !zero_padding_insertion_ind + 2
        end;
        let num_of_digits = !str_arg_l - !zero_padding_insertion_ind in
        if
          !alternate_form && fmt_spec = 'o'
          && not (!zero_padding_insertion_ind < !str_arg_l && Bytes.get tmp !zero_padding_insertion_ind = '0')
        then if (not !precision_specified) || !precision < num_of_digits + 1 then precision := num_of_digits + 1;
        if num_of_digits < !precision then number_of_zeros_to_pad := !precision - num_of_digits;
        if (not !justify_left) && !zero_padding then begin
          let n = to_i32 (!min_field_width - (!str_arg_l + !number_of_zeros_to_pad)) in
          if n > 0 then number_of_zeros_to_pad := !number_of_zeros_to_pad + n
        end
    | _ ->
        zero_padding := false;
        justify_left := true;
        min_field_width := 0;
        str_arg_off := !p;
        str_arg_l := if ch ed !p = '\000' then 0 else 1);
    if ch ed !p <> '\000' then incr p;
    (* the padding, the zeros, the text *)
    let pad c =
      let pn = to_i32 (!min_field_width - (!str_arg_l + !number_of_zeros_to_pad)) in
      if pn > 0 then fill c pn
    in
    if not !justify_left then pad (if !zero_padding then '0' else ' ');
    if !number_of_zeros_to_pad = 0 then zero_padding_insertion_ind := 0
    else begin
      let zn = to_i32 !zero_padding_insertion_ind in
      if zn > 0 then put !str_arg !str_arg_off zn;
      let zn = to_i32 !number_of_zeros_to_pad in
      if zn > 0 then fill '0' zn
    end;
    let sn = to_i32 (!str_arg_l - !zero_padding_insertion_ind) in
    if sn > 0 then put !str_arg (!str_arg_off + !zero_padding_insertion_ind) sn;
    if !justify_left then pad ' ';
    !p
  in
  if fmt <> 0 then begin
    let rec loop p =
      match ch ed p with
      | '\000' -> ()
      | '%' -> loop (conversion (p + 1))
      | _ ->
          let q = literal_end ed p in
          put ed.mem p (q - p);
          loop q
    in
    loop fmt
  end;
  if str_m > 0 then st_u8 ed (str + min !str_l (str_m - 1)) 0;
  to_i32 !str_l
