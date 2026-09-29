-- | caprice's launcher: the editor in Haskell run as a program -- its
-- arguments the C's argv, host_exit's status the process's.
module Main (main) where

import Caprice.Host (HostExit (..))
import Caprice.Rt
import qualified Caprice.Editor as Editor
import Control.Exception (catch)
import Control.Monad (forM_)
import qualified Data.ByteString as B
import qualified Data.ByteString.Char8 as BC
import qualified Data.ByteString.Unsafe as BU
import Foreign.Marshal.Alloc (callocBytes)
import Foreign.Marshal.Utils (copyBytes)
import System.Environment (getProgName)
import System.Exit (ExitCode (..), exitWith)
import System.Posix.Env.ByteString (getArgs)

main :: IO ()
main = do
  prog <- getProgName
  args <- getArgs
  ed <- Editor.newEditor
  let argv' = BC.pack prog : args
  argv <- callocBytes (8 * (length argv' + 1))
  forM_ (zip [0 ..] argv') $ \(i, a) -> do
    s <- callocBytes (B.length a + 1)
    BU.unsafeUseAsCStringLen a $ \(p, n) -> copyBytes (castPtr s) p n
    wrP argv (8 * i) s
  r <- Editor.vim_main ed (fromIntegral (length argv')) argv `catch` \(HostExit c) -> pure c
  exitWith (if r == 0 then ExitSuccess else ExitFailure (fromIntegral r))
