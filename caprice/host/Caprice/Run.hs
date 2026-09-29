-- | An editor run on a host: what an embedding program calls -- editor/host.go's
-- Main(host, args).  Each run is an editor of its own, its core's segment and
-- its host's arena, so a process may run several at once, each on its own
-- thread.
module Caprice.Run (run) where

import Caprice.Host (Host, HostExit (..), newEdHost)
import Caprice.Rt
import qualified Caprice.Editor as Editor
import Control.Exception (catch)
import Control.Monad (forM_)
import qualified Data.ByteString as B
import qualified Data.ByteString.Unsafe as BU
import Foreign.Marshal.Alloc (callocBytes)
import Foreign.Marshal.Utils (copyBytes)

-- | A new editor on h, run with args as the C's argv (the program's name
-- first) to its end: the status host_exit gave, or vim_main returned.
run :: Host -> [B.ByteString] -> IO Int32
run h args = do
  eh <- newEdHost h
  ed <- Editor.newEditor (toDyn eh)
  argv <- callocBytes (8 * (length args + 1))
  forM_ (zip [0 ..] args) $ \(i, a) -> do
    s <- callocBytes (B.length a + 1)
    BU.unsafeUseAsCStringLen a $ \(p, n) -> copyBytes (castPtr s) p n
    wrP argv (8 * i) s
  Editor.vim_main ed (fromIntegral (length args)) argv `catch` \(HostExit c) -> pure c
