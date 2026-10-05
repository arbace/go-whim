" whimview: the commands; the client is autoload/whimview.vim.
" whimview: a view of the program's graph edited in a vim buffer, through
" `go tool whim view-serve` (doc/GRAPH.md, *The vim client*).
"
" The server holds the graph, a session and one buffer of a view's text;
" this plugin keeps a vim buffer beside it.  Every change made in the vim
" buffer is sent as one `change` -- the bytes where it differs from the
" text the server holds -- and the answer says what it made: APPLIED (an
" edit of the graph, the view printed again: the buffer takes the print,
" the cursor where the change ended), PENDING (half-typed, the reason
" shown, the graph untouched), LAYOUT or SAME.  Typed changes are sent as
" they are made (TextChanged, TextChangedI) unless g:whimview_live is 0.
"
"   :WhimView [--ids] VIEW ARG   open a view (def NAME, callers F, uses NAME, ...)
"   :WhimSync                    send what changed (the autocommands do it)
"   :WhimAt [VIEW]               the view at the cursor: VIEW of what is under it
"   :WhimRename NAME             the entity at the cursor renamed, every use
"   :WhimUndo                    the graph's last edit undone
"   :WhimRevert                  what is pending dropped
"   :WhimC NAME                  NAME's definition as C, in a split
"   :WhimWrite FILE              the program's C written to FILE
"   :WhimStop                    the server stopped
"
"   g:whimview_cmd   the server's command (['go', 'tool', 'whim', 'view-serve'])
"   g:whimview_cwd   where it runs (the current directory)
"   g:whimview_file  the C file or graph it serves (its default, src/whim-vim.c)
"   g:whimview_live  1: send changes as they are typed (the default)

command! -nargs=+ WhimView call whimview#View(<q-args>)
command! WhimSync call whimview#Sync()
command! -nargs=? WhimAt call whimview#At(<q-args>)
command! -nargs=1 WhimRename call whimview#Rename(<q-args>)
command! WhimUndo call whimview#Undo()
command! WhimRevert call whimview#Revert()
command! -nargs=1 WhimC call whimview#C(<q-args>)
command! -nargs=1 -complete=file WhimWrite call whimview#Write(<q-args>)
command! WhimStop call whimview#Stop()
autocmd VimLeavePre * call whimview#Stop()
