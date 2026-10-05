vim9script
# whimview: a view of the program's graph edited in a vim buffer, through
# `go tool whim view-serve` (doc/GRAPH.md, *The vim client*).
#
# The server holds the graph, a session and its buffers of views' texts;
# this plugin keeps a vim buffer beside each (b:whim_id its number), any
# number of them open at once.  Every change made in a vim buffer is sent
# as one `change` -- the bytes where it differs from the
# text the server holds -- and the answer says what it made: APPLIED (an
# edit of the graph, the view printed again: the buffer takes the print,
# the cursor where the change ended), PENDING (half-typed, the reason
# shown, the graph untouched), LAYOUT or SAME.  Typed changes are sent as
# they are made (TextChanged, TextChangedI) unless g:whimview_live is 0.
# An edit through one view changes the graph under the others: those the
# server printed again (`touched`) are refreshed.
#
#   :WhimView[!] [--ids] VIEW ARG  a view in this window (def NAME, callers F, uses NAME,
#                                ...): in this whimview buffer, or a new one; refused while
#                                text is pending, unless !
#   :WhimSplit [--ids] VIEW ARG  a view in a new window, beside the others
#   :WhimSync                    send what changed (the autocommands do it)
#   :WhimAt [VIEW]               the view at the cursor: VIEW of what is under it
#   :WhimRename NAME             the entity at the cursor renamed, every use
#   :WhimUndo                    the graph's last edit undone
#   :WhimRevert                  what is pending dropped
#   :WhimC NAME                  NAME's definition as C, in a split
#   :WhimWrite FILE              the program's C written to FILE
#   :WhimStatus                  the last answer whole: a pending reason, cut when echoed
#   :WhimStop                    the server stopped
#
#   g:whimview_cmd   the server's command (['go', 'tool', 'whim', 'view-serve'])
#   g:whimview_cwd   where it runs (the current directory)
#   g:whimview_file  the C file or graph it serves (its default, src/whim-vim.c)
#   g:whimview_live  1: send changes as they are typed (the default)

var job: job = null_job
var chan: channel = null_channel
var bufs: dict<dict<any>> = {} # a vim buffer's: its server buffer id, the text held, its last newline
var syncing = false

def Start()
  if job_status(job) == 'run'
    return
  endif
  var cmd = copy(get(g:, 'whimview_cmd', ['go', 'tool', 'whim', 'view-serve']))
  if get(g:, 'whimview_file', '') != ''
    cmd += [g:whimview_file]
  endif
  job = job_start(cmd, {mode: 'raw', cwd: get(g:, 'whimview_cwd', getcwd()), err_io: 'null'})
  if job_status(job) != 'run'
    throw 'whimview: the server did not start: ' .. string(cmd)
  endif
  chan = job_getchannel(job)
enddef

# Quote is s as a Go string literal, the server's "..." word.
def Quote(s: string): string
  var q = escape(s, '\"')
  q = substitute(q, "\n", '\\n', 'g')
  q = substitute(q, "\r", '\\r', 'g')
  q = substitute(q, "\t", '\\t', 'g')
  return '"' .. q .. '"'
enddef

# Ask sends one request and returns [kind, head, body]: kind ok or error,
# head the header's words after the count.
def Ask(req: string): list<string>
  Start()
  ch_sendraw(chan, req .. "\n")
  var got = ''
  var deadline = reltime()
  while true
    var nlAt = stridx(got, "\n")
    if nlAt >= 0
      var header = strpart(got, 0, nlAt)
      var words = split(header, ' ', 1)
      var n = str2nr(words[1])
      var rest = strpart(got, nlAt + 1)
      # the body, a newline when it lacks one, ;;end and its newline
      var bodyEnds = n > 0 && strpart(rest, n - 1, 1) == "\n"
      var need = n + (n > 0 && !bodyEnds ? 1 : 0) + 6
      if len(rest) >= need
        return [words[0], join(words[2 :], ' '), strpart(rest, 0, n)]
      endif
    endif
    if reltimefloat(reltime(deadline)) > 30.0
      throw 'whimview: no answer to ' .. req
    endif
    got ..= ch_readraw(chan, {timeout: 100})
  endwhile
  return []
enddef

def Must(req: string): list<string>
  var r = Ask(req)
  if r[0] != 'ok'
    throw 'whimview: ' .. trim(r[2])
  endif
  return r
enddef

# B is the state of vim buffer nr, a whimview buffer.
def B(nr: number): dict<any>
  if !has_key(bufs, string(nr))
    throw 'whimview: not a whimview buffer: :WhimView VIEW ARG'
  endif
  return bufs[string(nr)]
enddef

# On is request req for vim buffer nr's server buffer.
def On(nr: number, req: string): list<string>
  return Must('@' .. B(nr).id .. ' ' .. req)
enddef

# Text is vim buffer nr's text, as the server's text is written.
def Text(nr: number): string
  var t = join(getbufline(nr, 1, '$'), "\n")
  return B(nr).nl ? t .. "\n" : t
enddef

# Show puts text in vim buffer nr, and, when it is the current one, the
# cursor at byte off of it.
def Show(nr: number, text: string, off: number)
  var st = B(nr)
  st.held = text
  st.nl = text =~ "\n$"
  var lines = split(text, "\n", 1)
  if st.nl
    remove(lines, -1)
  endif
  syncing = true
  if getbufline(nr, 1, '$') != lines
    setbufline(nr, 1, lines)
    deletebufline(nr, len(lines) + 1, '$')
  endif
  syncing = false
  if off >= 0 && bufnr() == nr
    var lnum = max([1, byte2line(min([off, len(text) - 1]) + 1)])
    cursor(lnum, off + 1 - line2byte(lnum) + 1)
  endif
enddef

# Refresh is the vim buffers whose server buffers an answer says were
# printed again (`touched B...` among its header's words).
def Refresh(words: list<string>)
  var at = index(words, 'touched')
  if at < 0
    return
  endif
  var ids: list<number> = []
  for w in words[at + 1 :]
    if w !~ '^\d\+$'
      break
    endif
    add(ids, str2nr(w))
  endfor
  for [k, st] in items(bufs)
    if index(ids, st.id) >= 0 && bufexists(str2nr(k))
      Show(str2nr(k), Must('@' .. st.id .. ' text')[2], -1)
    endif
  endfor
enddef

# Status keeps what the last answer said in b:whim_status (for a
# statusline) and echoes a pending reason, on one line cut to the screen's
# width: a message of two lines would stop vim at its hit-enter prompt,
# which takes the keys typed after it.  :WhimStatus shows it whole.
def Status(nr: number, s: string)
  setbufvar(nr, 'whim_status', s)
  if s =~ '^pending'
    echohl WarningMsg | echo strcharpart('whimview: ' .. s, 0, &columns - 12) | echohl None
  elseif s =~ '^\(renamed\|undone\|#\)'
    echo strcharpart('whimview: ' .. s, 0, &columns - 12)
  else
    echo ''
  endif
enddef

export def ShowStatus()
  echo 'whimview: ' .. getbufvar(bufnr(), 'whim_status', '')
enddef

# New makes the current window's buffer a new whimview buffer.
def New(): number
  enew
  setlocal buftype=nofile bufhidden=hide noswapfile filetype=lisp
  var nr = bufnr()
  augroup whimview
    autocmd! * <buffer>
    autocmd TextChanged,TextChangedI <buffer> if get(g:, 'whimview_live', 1) | call whimview#Sync() | endif
    autocmd BufWipeout <buffer> call whimview#Closed(str2nr(expand('<abuf>')))
  augroup END
  return nr
enddef

export def View(args: string, force: bool = false)
  var nr = bufnr()
  var r: list<string>
  if has_key(bufs, string(nr))
    if !force && getbufvar(nr, 'whim_status', '') =~ '^pending'
      throw 'whimview: the buffer has text pending (:WhimStatus); :WhimView! drops it, :WhimRevert first keeps the view'
    endif
    r = On(nr, 'open ' .. args)
  else
    r = Must('open ' .. args)
    nr = New()
    bufs[string(nr)] = {id: str2nr(split(r[1], ' ')[1]), held: '', nl: true}
  endif
  silent! execute 'file' fnameescape('whimview://' .. B(nr).id .. '/' .. args)
  Show(nr, r[2], 0)
  Status(nr, '')
enddef

# Split is a view in a new window: a buffer of its own on the session.
export def Split(args: string)
  new
  View(args)
enddef

# Closed is a whimview buffer wiped: its server buffer closed.
export def Closed(nr: number)
  if has_key(bufs, string(nr))
    Ask('@' .. bufs[string(nr)].id .. ' close')
    remove(bufs, string(nr))
  endif
enddef

# Sync sends what changed in the current buffer: the lines from the first
# that differs from the text the server holds to the last, whole -- the
# server finds the bytes within them -- found by comparing lines, which vim
# does natively (a string's index in vim9script counts characters from its
# start, and a loop over a view of thousands of lines by them is seconds a
# key).
export def Sync()
  var nr = bufnr()
  if syncing || !has_key(bufs, string(nr))
    return
  endif
  var st = B(nr)
  var cur = getbufline(nr, 1, '$')
  var old = Lines(st.held)
  if cur == old
    return
  endif
  var a = 0
  while a < len(cur) && a < len(old) && cur[a] == old[a]
    a += 1
  endwhile
  var q = 0
  while q < len(cur) - a && q < len(old) - a && cur[len(cur) - 1 - q] == old[len(old) - 1 - q]
    q += 1
  endwhile
  var from = a > 0 ? len(join(old[: a - 1], "\n")) + 1 : 0
  var was = Part(old, a, len(old) - q, st.nl)
  var now = Part(cur, a, len(cur) - q, st.nl)
  var here = line2byte(line('.')) + col('.') - 2
  var r = On(nr, printf('change %d %d %s %d', from, from + len(was), Quote(now), max([0, here])))
  var w = split(r[1], ' ')
  var status = w[0]
  if status == 'applied'
    Show(nr, r[2], str2nr(w[1]))
    var added = index(w, 'added')
    var names: list<string> = []
    if added >= 0
      for x in w[added + 1 :]
        if x == 'touched'
          break
        endif
        add(names, x)
      endfor
    endif
    Status(nr, added < 0 ? 'applied' : 'applied; added ' .. join(names, ', ') .. ' (:WhimView def NAME)')
    Refresh(w)
  else
    st.held = r[2]
    Status(nr, status == 'pending' ? 'pending: ' .. join(w[2 :], ' ') : '')
  endif
enddef

# Lines is a text as the buffer's lines.
def Lines(text: string): list<string>
  var ls = split(text, "\n", 1)
  if text =~ "\n$"
    remove(ls, -1)
  endif
  return ls
enddef

# Part is lines lo to hi of ls as they are in the text: each with the
# newline after it, but the text's last when the text ends without one.
def Part(ls: list<string>, lo: number, hi: number, nl: bool): string
  if hi <= lo
    return ''
  endif
  var t = join(ls[lo : hi - 1], "\n")
  return hi < len(ls) || nl ? t .. "\n" : t
enddef

export def At(view: string)
  Sync()
  var nr = bufnr()
  var off = line2byte(line('.')) + col('.') - 2
  var r = On(nr, 'at ' .. off .. (view == '' ? '' : ' ' .. view))
  silent! execute 'file' fnameescape('whimview://' .. B(nr).id .. '/at ' .. r[1])
  Show(nr, r[2], 0)
  Status(nr, r[1])
enddef

export def Rename(to: string)
  Sync()
  var nr = bufnr()
  var off = line2byte(line('.')) + col('.') - 2
  var r = On(nr, 'rename ' .. off .. ' ' .. to)
  var keep = getpos('.')
  Show(nr, r[2], -1)
  setpos('.', keep)
  Status(nr, 'renamed: ' .. r[1])
  Refresh(split(r[1], ' '))
enddef

export def Undo()
  var nr = bufnr()
  var r = On(nr, 'undo')
  Show(nr, r[2], -1)
  Status(nr, 'undone')
  Refresh(split(r[1], ' '))
enddef

export def Revert()
  var nr = bufnr()
  Show(nr, On(nr, 'revert')[2], -1)
  Status(nr, '')
enddef

export def C(name: string)
  var r = Must('c ' .. name)
  new
  setlocal buftype=nofile bufhidden=wipe noswapfile filetype=c
  setline(1, split(r[2], "\n"))
  silent! execute 'file' fnameescape('whimview://c ' .. name)
enddef

export def Write(file: string)
  Must('write ' .. file)
  echo 'whimview: ' .. file .. ' written'
enddef

export def Stop()
  if job_status(job) == 'run'
    Ask('quit')
    job_stop(job)
  endif
enddef

# State, for a test: the current buffer's text, what the server holds, and
# what it said; State(nr) another buffer's.
export def State(nr: number = 0): dict<any>
  var b = nr == 0 ? bufnr() : nr
  return {text: Text(b), held: B(b).held, status: getbufvar(b, 'whim_status', ''), id: B(b).id}
enddef
