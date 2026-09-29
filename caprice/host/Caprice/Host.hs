-- | The host the core runs on, as an interface: what the core asks of the
-- world it runs in (a terminal, a clock, input with a timeout, the signals,
-- output) is a Host, a record of functions, which an embedding program hands
-- Caprice.Run.run -- editor/host.go's Host, on raw buffers.  The core calls
-- the C host's 17 functions by name (host_write, musl_read_input, ...);
-- here each is a line of glue to its editor's Host, which Ed carries.  So a
-- process holds any number of editors, each on its own host: the terminal
-- (Caprice.Term) or anything else.
--
-- What is the editor's and not the Host's -- the arena, which the C host
-- keeps as the core's memory -- is kept here per editor, as editorHost is in
-- the Go.
module Caprice.Host
  ( Host (..)
  , EdHost (..)
  , HostExit (..)
  , newEdHost
  , host_alloc
  , host_exit
  , host_message
  , host_raise
  , host_time
  , host_write
  , musl_delay
  , musl_get_winsize
  , musl_host_init
  , musl_now_ms
  , musl_read_input
  , musl_suspend
  , musl_term_start
  , musl_term_stop
  , musl_tty_keys
  , musl_wait_for_input
  , vim_snprintf
  ) where

import Caprice.Printf (vim_snprintf)
import Caprice.Rt
import {-# SOURCE #-} Caprice.Editor (deathtrap)
import Control.Exception (Exception, throwIO)
import Control.Monad (when)
import Data.Char (ord)
import Data.IORef
import Foreign.Marshal.Alloc (callocBytes)

-- | What the core needs of the world it runs in: editor/host.go's Host.
data Host = Host
  { -- | Start catching the signals the editor handles; the host calls the
    -- core's deathtrap with a SIGHUP's or SIGTERM's number, on the editor's
    -- thread, where the C handler would have run.
    hInit :: (Int32 -> IO ()) -> IO ()
  , -- | The terminal's rows and columns, when it has them.
    hWinSize :: IO (Maybe (Int32, Int32))
  , -- | Raw mode, and out of it.
    hTermStart, hTermStop :: IO ()
  , -- | fd's erase and interrupt characters and whether it maps CR to NL
    -- and NL to CR NL, when fd is a terminal.
    hTtyKeys :: Int32 -> IO (Maybe (Int32, Int32, Bool, Bool))
  , -- | Milliseconds of a monotonic clock, and the Unix time.
    hNowMs, hTime :: IO Int64
  , -- | Sleep ms milliseconds; interruptible lets the terminal relax meanwhile.
    hDelay :: Int64 -> Bool -> IO ()
  , -- | Whether input is waiting within ms (forever when negative); true also
    -- for a signal the core must hear of.
    hWaitForInput :: Int64 -> IO Bool
  , -- | Up to len bytes of input at buf, how many; a signal's own bytes first.
    hReadInput :: P -> Int32 -> IO Int32
  , -- | Raise sig in this editor's process, and suspend it.
    hRaise :: Int32 -> IO ()
  , hSuspend :: IO ()
  , -- | A message outside the screen: len bytes of msg (to its NUL when len
    -- is negative), an error's to the error stream.
    hMessage :: P -> Int32 -> Bool -> IO ()
  , -- | len bytes of s to the screen, how many were written.
    hWrite :: P -> Int32 -> IO Int32
  }

-- | host_exit: the editor ends with this status, where Caprice.Run.run
-- catches it -- the C's longjmp to main.
newtype HostExit = HostExit Int32 deriving (Show)

instance Exception HostExit

-- | What an editor's Ed carries: its Host, and its arena.
data EdHost = EdHost {ehHost :: !Host, ehArena :: !(IORef (P, Int))}

-- | An editor's glue to h, with an arena of its own.
newEdHost :: Host -> IO EdHost
newEdHost h = do
  a <- callocBytes arenaBytes
  EdHost h <$> newIORef (a, 0)

-- | The C host's arena: 1 GiB, zeroed, never freed; calloc gives the pages
-- as they are touched, so an editor that uses little costs little.
arenaBytes :: Int
arenaBytes = 1024 * 1024 * 1024

glue :: Ed -> EdHost
glue ed = fromDyn (edHost ed) (error "caprice: an editor with no host (Caprice.Run.run makes one)")
{-# INLINE glue #-}

host :: Ed -> Host
host = ehHost . glue
{-# INLINE host #-}

musl_host_init :: Ed -> IO ()
musl_host_init ed = hInit (host ed) (deathtrap ed)

musl_get_winsize :: Ed -> P -> P -> IO Int32
musl_get_winsize ed rows cols = do
  s <- hWinSize (host ed)
  case s of
    Just (r, c) -> wrI32 rows 0 r >> wrI32 cols 0 c >> pure 1
    Nothing -> pure 0

musl_term_start, musl_term_stop, musl_suspend :: Ed -> IO ()
musl_term_start ed = hTermStart (host ed)
musl_term_stop ed = hTermStop (host ed)
musl_suspend ed = hSuspend (host ed)

musl_tty_keys :: Ed -> Int32 -> P -> P -> P -> P -> IO Int32
musl_tty_keys ed fd bs intr cr nlcr = do
  k <- hTtyKeys (host ed) fd
  case k of
    Nothing -> pure 0
    Just (e, i, c, n) -> do
      wrI32 bs 0 e
      wrI32 intr 0 i
      wrI32 cr 0 (b2i c)
      wrI32 nlcr 0 (b2i n)
      pure 1

musl_now_ms, host_time :: Ed -> IO Int64
musl_now_ms ed = hNowMs (host ed)
host_time ed = hTime (host ed)

musl_delay :: Ed -> Int64 -> Int32 -> IO ()
musl_delay ed ms interruptible = hDelay (host ed) ms (interruptible /= 0)

musl_wait_for_input :: Ed -> Int64 -> IO Int32
musl_wait_for_input ed ms = b2i <$> hWaitForInput (host ed) ms

musl_read_input :: Ed -> P -> Int32 -> IO Int32
musl_read_input ed = hReadInput (host ed)

host_raise :: Ed -> Int32 -> IO ()
host_raise ed = hRaise (host ed)

host_exit :: Ed -> Int32 -> IO ()
host_exit _ r = throwIO (HostExit r)

host_message :: Ed -> P -> Int32 -> Int32 -> IO ()
host_message ed msg len err = hMessage (host ed) msg len (err /= 0)

host_write :: Ed -> P -> Int32 -> IO Int32
host_write ed = hWrite (host ed)

-- | n bytes of the editor's arena, aligned as max_align_t is; its exhaustion
-- ends the editor, as the C's ends the process.  The count is added to
-- atomically: the regex engine may run on several threads at once (chunks,
-- match_lines), each allocating.
host_alloc :: Ed -> Word64 -> IO P
host_alloc ed n = do
  let want = (n + 15) .&. complement 15
  (base, used) <- atomicModifyIORef' (ehArena (glue ed)) (\(b, u) -> ((b, u + fromIntegral want), (b, u)))
  when (want < n || want > fromIntegral (arenaBytes - used)) $ do
    m <- lit ("whim-vim: host arena exhausted: " ++ show arenaBytes ++ " bytes, " ++ show used ++ " used, request " ++ show n ++ "\n")
    host_message ed m (-1) 1
    host_exit ed 1
  pure (pAdd base used)

lit :: String -> IO P
lit s = do
  p <- callocBytes (length s + 1)
  mapM_ (\(k, c) -> wrW8 p k (fromIntegral (ord c))) (zip [0 ..] s)
  pure p
