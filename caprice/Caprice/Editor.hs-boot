module Caprice.Editor where

import Caprice.Rt

deathtrap :: Ed -> Int32 -> IO ()
emsg :: Ed -> P -> IO Int32
iemsg :: Ed -> P -> IO ()
emsg_iobuff_room :: Ed -> IO Word64
iobuff_or :: Ed -> P -> IO P
utfc_ptr2len :: Ed -> P -> IO Int32
utf_ptr2cells :: Ed -> P -> IO Int32
addr'IObuff :: Ed -> P
