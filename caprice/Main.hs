-- | caprice's launcher: the editor in Haskell run as a program on the
-- terminal -- its arguments the C's argv, host_exit's status the process's.
module Main (main) where

import qualified Caprice.Run as Run
import qualified Caprice.Term as Term
import qualified Data.ByteString.Char8 as BC
import System.Environment (getProgName)
import System.Exit (ExitCode (..), exitWith)
import System.Posix.Env.ByteString (getArgs)

main :: IO ()
main = do
  prog <- getProgName
  args <- getArgs
  h <- Term.newTerm
  r <- Run.run h (BC.pack prog : args)
  exitWith (if r == 0 then ExitSuccess else ExitFailure (fromIntegral r))
