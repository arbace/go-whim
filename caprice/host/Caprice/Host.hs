{-# LANGUAGE CApiFFI #-}

-- | The host caprice's core runs on: what the core asks of an operating
-- system, from the C host half of whim-vim.c, function by function -- the
-- terminal (raw mode, the window's size, the keys), the input with a
-- timeout, the signals, output, the clock, the arena, and the exit.  One
-- editor per process: the host's state is the process's.
--
-- The C's signal handlers set a flag and write to a pipe that wakes the
-- select waiting for input; here too, the wait a poll(2) on the keys and
-- the pipe -- a safe foreign call, which lets the handlers, Haskell threads
-- of the threaded RTS, run meanwhile.  host_exit's longjmp to main is an
-- exception main catches.
module Caprice.Host
  ( host_alloc
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
  , HostExit (..)
  ) where

import Caprice.Printf (vim_snprintf)
import Caprice.Rt
import {-# SOURCE #-} Caprice.Editor (deathtrap)
import Control.Concurrent (threadDelay)
import Control.Exception (Exception, IOException, SomeException, throwIO, try)
import Control.Monad (void, when)
import Data.Char (ord)
import Data.IORef
import Data.Time.Clock.POSIX (getPOSIXTime)
import Foreign.C.Types (CInt (..), CShort (..), CULong (..), CUShort)
import Foreign.Marshal.Alloc (allocaBytes, callocBytes)
import Foreign.Storable (peekByteOff)
import Foreign.C.Error (eINTR, getErrno)
import System.Environment (lookupEnv)
import System.IO.Unsafe (unsafePerformIO)
import System.Posix.IO (FdOption (..), createPipe, fdReadBuf, fdWrite, fdWriteBuf, setFdOption, stdError, stdInput, stdOutput)
import System.Posix.Process (getProcessID)
import System.Posix.Signals
import System.Posix.Terminal
import System.Posix.Types (ByteCount, Fd (..))

-- | host_exit: the process ends with this status, where main catches it.
newtype HostExit = HostExit Int32 deriving (Show)

instance Exception HostExit

data St = St
  { stWinch, stTstp, stInt :: IORef Bool
  , stDeath :: IORef Int32
  , stPipe :: IORef (Maybe (Fd, Fd))
  , stSaved :: IORef (Maybe TerminalAttributes)
  , stRaw :: IORef Bool
  , stNowBase :: IORef (Maybe Integer)
  , stArena :: IORef (P, Int)
  }

-- | The C host's static state.
st :: St
st = unsafePerformIO $ do
  arena <- callocBytes arenaBytes
  St <$> newIORef False <*> newIORef False <*> newIORef False <*> newIORef 0 <*> newIORef Nothing
    <*> newIORef Nothing <*> newIORef False <*> newIORef Nothing <*> newIORef (arena, 0)
{-# NOINLINE st #-}

-- | The C host's arena: 1 GiB, zeroed, never freed; calloc gives the pages
-- as they are touched.
arenaBytes :: Int
arenaBytes = 1024 * 1024 * 1024

-- | A byte down the pipe the wait for input watches: the C handler's
-- write, which wakes its select.
wake :: IO ()
wake = do
  p <- readIORef (stPipe st)
  case p of
    Just (_, w) -> void (try (fdWrite w "x") :: IO (Either IOException ByteCount))
    Nothing -> pure ()

onFlag :: IORef Bool -> Handler
onFlag r = Catch (writeIORef r True >> wake)

onDeath :: Signal -> Handler
onDeath s = Catch (writeIORef (stDeath st) (fromIntegral s) >> wake)

musl_host_init :: Ed -> IO ()
musl_host_init _ = do
  pipe <- try createPipe :: IO (Either IOException (Fd, Fd))
  case pipe of
    Right (r, w) -> do
      mapM_ (\fd -> setFdOption fd NonBlockingRead True >> setFdOption fd CloseOnExec True) [r, w]
      writeIORef (stPipe st) (Just (r, w))
    Left _ -> pure ()
  void $ installHandler sigHUP (onDeath sigHUP) Nothing
  void $ installHandler sigTERM (onDeath sigTERM) Nothing
  void $ installHandler sigWINCH (onFlag (stWinch st)) Nothing
  void $ installHandler sigCONT (onFlag (stWinch st)) Nothing
  void $ installHandler sigTSTP (onFlag (stTstp st)) Nothing
  void $ installHandler sigINT (onFlag (stInt st)) Nothing
  void $ installHandler sigPIPE Ignore Nothing
  void $ installHandler sigALRM Ignore Nothing

-- | A pending SIGHUP or SIGTERM, to the core's deathtrap.
deliverDeath :: Ed -> IO ()
deliverDeath ed = do
  p <- readIORef (stPipe st)
  case p of
    Just (r, _) -> allocaBytes 16 $ \b ->
      let drain = do
            n <- try (fdReadBuf r b 16) :: IO (Either IOException ByteCount)
            case n of
              Right k | k > 0 -> drain
              _ -> pure ()
       in drain
    Nothing -> pure ()
  sig <- readIORef (stDeath st)
  when (sig /= 0) $ do
    writeIORef (stDeath st) 0
    deathtrap ed sig

ttySet :: Bool -> Bool -> IO ()
ttySet raw sleep = do
  saved <- readIORef (stSaved st)
  base <- case saved of
    Just a -> pure (Just a)
    Nothing -> do
      r <- try (getTerminalAttributes stdInput) :: IO (Either SomeException TerminalAttributes)
      case r of
        Left _ -> pure Nothing
        Right a -> writeIORef (stSaved st) (Just a) >> pure (Just a)
  case base of
    Nothing -> pure ()
    Just a -> do
      let without m as = foldl withoutMode as m
          new
            | raw =
                withTime (withMinInput (without [MapCRtoLF, StartStopOutput, ProcessInput, EnableEcho, KeyboardInterrupts, EchoErase, ExtendedFunctions, MapLFtoCRLF, TabDelayMask3] a) 1) 0
            | sleep = withTime (withMinInput (without [ProcessInput, EnableEcho] a) 1) 0
            | otherwise = a
      void (try (setTerminalAttributes stdInput new Immediately) :: IO (Either SomeException ()))

foreign import capi "signal.h value SIGWINCH" sigWINCH :: CInt

foreign import capi "sys/ioctl.h ioctl" c_ioctl :: CInt -> CULong -> P -> IO CInt

foreign import capi "sys/ioctl.h value TIOCGWINSZ" c_TIOCGWINSZ :: CULong

-- | The window's rows and columns, OK (1), or FAIL (0).
musl_get_winsize :: Ed -> P -> P -> IO Int32
musl_get_winsize _ rows cols = allocaBytes 8 $ \ws -> do
  r <- c_ioctl 1 c_TIOCGWINSZ ws
  row <- peekByteOff ws 0 :: IO CUShort
  col <- peekByteOff ws 2 :: IO CUShort
  if r /= 0 || row == 0 || col == 0
    then pure 0
    else do
      wrI32 rows 0 (fromIntegral row)
      wrI32 cols 0 (fromIntegral col)
      pure 1

musl_term_start :: Ed -> IO ()
musl_term_start _ = writeIORef (stRaw st) True >> ttySet True False

musl_term_stop :: Ed -> IO ()
musl_term_stop _ = writeIORef (stRaw st) False >> ttySet False False

musl_tty_keys :: Ed -> Int32 -> P -> P -> P -> P -> IO Int32
musl_tty_keys _ fd bs intr cr nlcr = do
  r <- try (getTerminalAttributes (Fd (fromIntegral fd))) :: IO (Either SomeException TerminalAttributes)
  case r of
    Left _ -> pure 0
    Right a -> do
      let cc c = maybe 0 (fromIntegral . ord) (controlChar a c)
      wrI32 bs 0 (cc Erase)
      wrI32 intr 0 (cc Interrupt)
      wrI32 cr 0 (b2i (terminalMode MapCRtoLF a))
      wrI32 nlcr 0 (b2i (terminalMode MapLFtoCRLF a))
      pure 1

-- | Milliseconds since the second of the first call, as the C's
-- gettimeofday arithmetic has them.
musl_now_ms :: Ed -> IO Int64
musl_now_ms _ = do
  t <- getPOSIXTime
  let us = floor (t * 1000000) :: Integer
      sec = us `div` 1000000
  base <- readIORef (stNowBase st)
  b <- case base of
    Just b -> pure b
    Nothing -> writeIORef (stNowBase st) (Just sec) >> pure sec
  pure (fromIntegral ((sec - b) * 1000 + (us `mod` 1000000) `div` 1000))

-- | The time: WHIM_TIME, when the environment holds it (phase 180), as
-- atol reads it; the clock's otherwise.
host_time :: Ed -> IO Int64
host_time _ = do
  pinned <- lookupEnv "WHIM_TIME"
  case pinned of
    Just s@(_ : _) -> pure (atol s)
    _ -> floor <$> getPOSIXTime
  where
    atol s = case s of
      ('-' : r) -> negate (digits r)
      ('+' : r) -> digits r
      r -> digits r
    digits = foldl (\n c -> n * 10 + fromIntegral (ord c - 48)) 0 . takeWhile (\c -> c >= '0' && c <= '9')

musl_delay :: Ed -> Int64 -> Int32 -> IO ()
musl_delay _ ms interruptible = do
  raw <- readIORef (stRaw st)
  let relax = interruptible /= 0 && raw && ms > 500
  when relax $ ttySet False True
  when (ms > 0) $ threadDelay (fromIntegral ms * 1000)
  when relax $ ttySet True False

-- | Whether input is waiting within ms (forever when negative): 1 also
-- for a signal the core must hear of.  poll(2) on the keys and the pipe the
-- signal handlers write to, as the C's select: the kernel's answer, now.
musl_wait_for_input :: Ed -> Int64 -> IO Int32
musl_wait_for_input ed ms = allocaBytes 16 $ \fds -> do
  pipe <- readIORef (stPipe st)
  let loop = do
        deliverDeath ed
        w <- readIORef (stWinch st)
        t <- readIORef (stTstp st)
        i <- readIORef (stInt st)
        if w || t || i
          then pure 1
          else do
            -- struct pollfd { int fd; short events; short revents; }
            wrI32 fds 0 0
            wrI16 fds 4 pollIn
            wrI16 fds 6 0
            n <- case pipe of
              Just (Fd r, _) -> do
                wrI32 fds 8 (fromIntegral r)
                wrI16 fds 12 pollIn
                wrI16 fds 14 0
                pure 2
              Nothing -> pure 1
            ret <- c_poll fds n (fromIntegral (if ms >= 0 then ms else -1))
            if ret < 0
              then do
                e <- getErrno
                if e == eINTR then loop else pure 0
              else do
                pr <- if n == 2 then rdI16 fds 14 else pure 0
                if ret > 0 && pr /= 0
                  then loop
                  else do
                    r0 <- rdI16 fds 6
                    pure (b2i (ret > 0 && r0 /= 0))
  loop

foreign import capi safe "poll.h poll" c_poll :: P -> CULong -> CInt -> IO CInt

foreign import capi "poll.h value POLLIN" c_POLLIN :: CShort

pollIn :: Int16
pollIn = fromIntegral c_POLLIN

musl_read_input :: Ed -> P -> Int32 -> IO Int32
musl_read_input ed buf len = do
  deliverDeath ed
  i <- readIORef (stInt st)
  if i && len >= 1
    then do
      writeIORef (stInt st) False
      wrW8 buf 0 3
      pure 1
    else do
      when i $ writeIORef (stInt st) False
      w <- readIORef (stWinch st)
      winch <-
        if w
          then do
            writeIORef (stWinch st) False
            allocaBytes 8 $ \rc -> do
              ok <- musl_get_winsize ed rc (pAdd rc 4)
              rows <- rdI32 rc 0
              cols <- rdI32 rc 4
              if ok == 1 && len >= 32
                then do
                  fmt <- lit "\ESC[48;%d;%d;0;0t"
                  Just <$> vim_snprintf ed buf (fromIntegral len) fmt [VI (fromIntegral rows), VI (fromIntegral cols)]
                else pure Nothing
          else pure Nothing
      case winch of
        Just n -> pure n
        Nothing -> do
          t <- readIORef (stTstp st)
          if t
            then do
              writeIORef (stTstp st) False
              if len >= 5
                then do
                  mapM_ (\(k, c) -> wrW8 buf k c) (zip [0 ..] [27, 91, 63, 49, 122])
                  pure 5
                else readIn
            else readIn
  where
    readIn = do
      r <- try (fdReadBuf stdInput (castPtr buf) (fromIntegral len)) :: IO (Either IOException ByteCount)
      pure $ case r of
        Left _ -> -1
        Right n -> fromIntegral n

lit :: String -> IO P
lit s = do
  p <- callocBytes (length s + 1)
  mapM_ (\(k, c) -> wrW8 p k (fromIntegral (ord c))) (zip [0 ..] s)
  pure p

host_raise :: Ed -> Int32 -> IO ()
host_raise _ sig = getProcessID >>= signalProcess (fromIntegral sig)

musl_suspend :: Ed -> IO ()
musl_suspend _ = do
  void $ installHandler sigTSTP Default Nothing
  signalProcessGroup sigTSTP 0
  void $ installHandler sigTSTP (onFlag (stTstp st)) Nothing

host_exit :: Ed -> Int32 -> IO ()
host_exit _ r = throwIO (HostExit r)

-- | msg's len bytes (to its NUL when len is negative), to stderr when err.
host_message :: Ed -> P -> Int32 -> Int32 -> IO ()
host_message _ msg len err = do
  n <- if len < 0 then cLen msg else pure (fromIntegral len)
  let fd = if err /= 0 then stdError else stdOutput
      go off = when (off < n) $ do
        r <- try (fdWriteBuf fd (castPtr (pAdd msg off)) (fromIntegral (n - off))) :: IO (Either IOException ByteCount)
        case r of
          Right w | w > 0 -> go (off + fromIntegral w)
          _ -> pure ()
  go 0
  where
    cLen p = let go i = do c <- rdW8 p i; if c == 0 then pure i else go (i + 1) in go 0

-- | n bytes of the arena, aligned as max_align_t is; its exhaustion ends
-- the process, as the C's does.
-- The count is added to atomically: the regex engine may run on several
-- threads at once (chunks, match_lines), each allocating.
host_alloc :: Ed -> Word64 -> IO P
host_alloc ed n = do
  let want = (n + 15) .&. complement 15
  (base, used) <- atomicModifyIORef' (stArena st) (\(b, u) -> ((b, u + fromIntegral want), (b, u)))
  when (want < n || want > fromIntegral (arenaBytes - used)) $ do
    m <- lit ("whim-vim: host arena exhausted: " ++ show arenaBytes ++ " bytes, " ++ show used ++ " used, request " ++ show n ++ "\n")
    host_message ed m (-1) 1
    host_exit ed 1
  pure (pAdd base used)

host_write :: Ed -> P -> Int32 -> IO Int32
host_write _ s len = do
  r <- try (fdWriteBuf stdOutput (castPtr s) (fromIntegral len)) :: IO (Either IOException ByteCount)
  pure $ case r of
    Left _ -> -1
    Right n -> fromIntegral n
