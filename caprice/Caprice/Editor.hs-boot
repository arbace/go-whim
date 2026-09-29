module Caprice.Editor where

import Caprice.Rt

deathtrap :: Ed -> Int32 -> IO ()
emsg :: Ed -> Ptr Int8 -> IO Int32
iemsg :: Ed -> Ptr Int8 -> IO ()
emsg_iobuff_room :: Ed -> IO Word64
iobuff_or :: Ed -> Ptr Int8 -> IO (Ptr Int8)
utfc_ptr2len :: Ed -> Ptr Word8 -> IO Int32
utf_ptr2cells :: Ed -> Ptr Word8 -> IO Int32
addr'IObuff :: Ed -> Ptr a
