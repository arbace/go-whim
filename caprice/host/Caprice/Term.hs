{-# LANGUAGE CApiFFI #-}

-- | The terminal host: a Caprice.Host.Host on a terminal's file descriptors,
-- from the C host half of whim-vim.c function by function -- raw mode, the
-- window's size and the keys (System.Posix.Terminal, an ioctl), the wait for
-- input, the signals, output, the clocks.  editor/term's Host, in Haskell.
--
-- Its state is the instance's: a process may hold several, each an editor's.
-- The signals are the process's, though, so one handler per signal fans each
-- out to every terminal alive: each sets its flag and writes to its pipe,
-- which its wait for input polls beside the keys, as the C's handlers wake
-- its select.
module Caprice.Term (newTerm, newTermOn) where

import Caprice.Host (Host (..))
import Caprice.Rt
import Control.Concurrent (threadDelay)
import Control.Exception (IOException, SomeException, try)
import Control.Monad (void, when)
import Data.Char (ord)
import Data.IORef
import Data.Time.Clock.POSIX (getPOSIXTime)
import Foreign.C.Error (eINTR, getErrno)
import Foreign.C.Types (CInt (..), CShort (..), CULong (..), CUShort)
import Foreign.Marshal.Alloc (allocaBytes)
import Foreign.Storable (peekByteOff)
import System.Environment (lookupEnv)
import System.IO.Unsafe (unsafePerformIO)
import System.Posix.IO (FdOption (..), createPipe, fdReadBuf, fdWrite, fdWriteBuf, setFdOption, stdError, stdInput, stdOutput)
import System.Posix.Process (getProcessID)
import System.Posix.Signals
import System.Posix.Terminal
import System.Posix.Types (ByteCount, Fd (..))

-- | One terminal's state.
data Term = Term
  { tIn, tOut, tErr :: !Fd
  , tWinch, tTstp, tInt :: !(IORef Bool)
  , tDeath :: !(IORef Int32)
  , tPipe :: !(IORef (Maybe (Fd, Fd)))
  , tSaved :: !(IORef (Maybe TerminalAttributes))
  , tRaw :: !(IORef Bool)
  , tNowBase :: !(IORef (Maybe Integer))
  , tDeathtrap :: !(IORef (Int32 -> IO ()))
  }

-- | The terminals alive in the process: the signals' handlers tell each.
terms :: IORef [Term]
terms = unsafePerformIO (newIORef [])
{-# NOINLINE terms #-}

-- | Whether the process's handlers are installed.
installed :: IORef Bool
installed = unsafePerformIO (newIORef False)
{-# NOINLINE installed #-}

-- | The terminal host on stdin, stdout and stderr: the C host's.
newTerm :: IO Host
newTerm = newTermOn stdInput stdOutput stdError

-- | The terminal host on the given input, output and error descriptors.
newTermOn :: Fd -> Fd -> Fd -> IO Host
newTermOn i o e = do
  t <-
    Term i o e <$> newIORef False <*> newIORef False <*> newIORef False <*> newIORef 0 <*> newIORef Nothing
      <*> newIORef Nothing <*> newIORef False <*> newIORef Nothing <*> newIORef (\_ -> pure ())
  pure
    Host
      { hInit = initTerm t
      , hWinSize = winSize t
      , hTermStart = writeIORef (tRaw t) True >> ttySet t True False
      , hTermStop = writeIORef (tRaw t) False >> ttySet t False False
      , hTtyKeys = ttyKeys
      , hNowMs = nowMs t
      , hTime = unixTime
      , hDelay = delay t
      , hWaitForInput = waitForInput t
      , hReadInput = readInput t
      , hRaise = \sig -> getProcessID >>= signalProcess (fromIntegral sig)
      , hSuspend = suspend
      , hMessage = message t
      , hWrite = \s len -> do
          r <- try (fdWriteBuf (tOut t) (castPtr s) (fromIntegral len)) :: IO (Either IOException ByteCount)
          pure (either (const (-1)) fromIntegral r)
      }

-- | A byte down the terminal's pipe, which its wait for input watches.
wake :: Term -> IO ()
wake t = do
  p <- readIORef (tPipe t)
  case p of
    Just (_, w) -> void (try (fdWrite w "x") :: IO (Either IOException ByteCount))
    Nothing -> pure ()

-- | A signal's handler: every terminal alive told.
onEach :: (Term -> IO ()) -> Handler
onEach f = Catch $ readIORef terms >>= mapM_ (\t -> f t >> wake t)

initTerm :: Term -> (Int32 -> IO ()) -> IO ()
initTerm t deathtrap = do
  writeIORef (tDeathtrap t) deathtrap
  pipe <- try createPipe :: IO (Either IOException (Fd, Fd))
  case pipe of
    Right (r, w) -> do
      mapM_ (\fd -> setFdOption fd NonBlockingRead True >> setFdOption fd CloseOnExec True) [r, w]
      writeIORef (tPipe t) (Just (r, w))
    Left _ -> pure ()
  atomicModifyIORef' terms (\ts -> (t : ts, ()))
  first <- atomicModifyIORef' installed (\b -> (True, not b))
  when first $ do
    let death s = onEach (\x -> writeIORef (tDeath x) (fromIntegral s))
    void $ installHandler sigHUP (death sigHUP) Nothing
    void $ installHandler sigTERM (death sigTERM) Nothing
    void $ installHandler sigWINCH (onEach (\x -> writeIORef (tWinch x) True)) Nothing
    void $ installHandler sigCONT (onEach (\x -> writeIORef (tWinch x) True)) Nothing
    void $ installHandler sigTSTP onTstp Nothing
    void $ installHandler sigINT (onEach (\x -> writeIORef (tInt x) True)) Nothing
    void $ installHandler sigPIPE Ignore Nothing
    void $ installHandler sigALRM Ignore Nothing

onTstp :: Handler
onTstp = onEach (\x -> writeIORef (tTstp x) True)

-- | A pending SIGHUP or SIGTERM, to the core's deathtrap.
deliverDeath :: Term -> IO ()
deliverDeath t = do
  p <- readIORef (tPipe t)
  case p of
    Just (r, _) -> allocaBytes 16 $ \b ->
      let drain = do
            n <- try (fdReadBuf r b 16) :: IO (Either IOException ByteCount)
            case n of
              Right k | k > 0 -> drain
              _ -> pure ()
       in drain
    Nothing -> pure ()
  sig <- readIORef (tDeath t)
  when (sig /= 0) $ do
    writeIORef (tDeath t) 0
    dt <- readIORef (tDeathtrap t)
    dt sig

ttySet :: Term -> Bool -> Bool -> IO ()
ttySet t raw sleep = do
  saved <- readIORef (tSaved t)
  base <- case saved of
    Just a -> pure (Just a)
    Nothing -> do
      r <- try (getTerminalAttributes (tIn t)) :: IO (Either SomeException TerminalAttributes)
      case r of
        Left _ -> pure Nothing
        Right a -> writeIORef (tSaved t) (Just a) >> pure (Just a)
  case base of
    Nothing -> pure ()
    Just a -> do
      let without m as = foldl withoutMode as m
          new
            | raw =
                withTime (withMinInput (without [MapCRtoLF, StartStopOutput, ProcessInput, EnableEcho, KeyboardInterrupts, EchoErase, ExtendedFunctions, MapLFtoCRLF, TabDelayMask3] a) 1) 0
            | sleep = withTime (withMinInput (without [ProcessInput, EnableEcho] a) 1) 0
            | otherwise = a
      void (try (setTerminalAttributes (tIn t) new Immediately) :: IO (Either SomeException ()))

foreign import capi "signal.h value SIGWINCH" sigWINCH :: CInt

foreign import capi "sys/ioctl.h ioctl" c_ioctl :: CInt -> CULong -> P -> IO CInt

foreign import capi "sys/ioctl.h value TIOCGWINSZ" c_TIOCGWINSZ :: CULong

-- | The window's rows and columns, asked of the output.
winSize :: Term -> IO (Maybe (Int32, Int32))
winSize t = allocaBytes 8 $ \ws -> do
  let Fd o = tOut t
  r <- c_ioctl o c_TIOCGWINSZ ws
  row <- peekByteOff ws 0 :: IO CUShort
  col <- peekByteOff ws 2 :: IO CUShort
  pure (if r /= 0 || row == 0 || col == 0 then Nothing else Just (fromIntegral row, fromIntegral col))

ttyKeys :: Int32 -> IO (Maybe (Int32, Int32, Bool, Bool))
ttyKeys fd = do
  r <- try (getTerminalAttributes (Fd (fromIntegral fd))) :: IO (Either SomeException TerminalAttributes)
  pure $ case r of
    Left _ -> Nothing
    Right a ->
      let cc c = maybe 0 (fromIntegral . ord) (controlChar a c)
       in Just (cc Erase, cc Interrupt, terminalMode MapCRtoLF a, terminalMode MapLFtoCRLF a)

-- | Milliseconds since the second of the first call, as the C's
-- gettimeofday arithmetic has them.
nowMs :: Term -> IO Int64
nowMs t = do
  now <- getPOSIXTime
  let us = floor (now * 1000000) :: Integer
      sec = us `div` 1000000
  base <- readIORef (tNowBase t)
  b <- case base of
    Just b -> pure b
    Nothing -> writeIORef (tNowBase t) (Just sec) >> pure sec
  pure (fromIntegral ((sec - b) * 1000 + (us `mod` 1000000) `div` 1000))

-- | The time: WHIM_TIME, when the environment holds it (phase 99), as
-- atol reads it; the clock's otherwise.
unixTime :: IO Int64
unixTime = do
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

delay :: Term -> Int64 -> Bool -> IO ()
delay t ms interruptible = do
  raw <- readIORef (tRaw t)
  let relax = interruptible && raw && ms > 500
  when relax $ ttySet t False True
  when (ms > 0) $ threadDelay (fromIntegral ms * 1000)
  when relax $ ttySet t True False

-- | Whether input is waiting within ms (forever when negative); true also
-- for a signal the core must hear of.  poll(2) on the input and the pipe the
-- signals' handlers write to, as the C's select: the kernel's answer, now.
waitForInput :: Term -> Int64 -> IO Bool
waitForInput t ms = allocaBytes 16 $ \fds -> do
  pipe <- readIORef (tPipe t)
  let Fd i = tIn t
      loop = do
        deliverDeath t
        w <- readIORef (tWinch t)
        s <- readIORef (tTstp t)
        n <- readIORef (tInt t)
        if w || s || n
          then pure True
          else do
            -- struct pollfd { int fd; short events; short revents; }
            wrI32 fds 0 (fromIntegral i)
            wrI16 fds 4 pollIn
            wrI16 fds 6 0
            k <- case pipe of
              Just (Fd r, _) -> do
                wrI32 fds 8 (fromIntegral r)
                wrI16 fds 12 pollIn
                wrI16 fds 14 0
                pure 2
              Nothing -> pure 1
            ret <- c_poll fds k (fromIntegral (if ms >= 0 then ms else -1))
            if ret < 0
              then do
                e <- getErrno
                if e == eINTR then loop else pure False
              else do
                pr <- if k == 2 then rdI16 fds 14 else pure 0
                if ret > 0 && pr /= 0
                  then loop
                  else do
                    r0 <- rdI16 fds 6
                    pure (ret > 0 && r0 /= 0)
  loop

foreign import capi safe "poll.h poll" c_poll :: P -> CULong -> CInt -> IO CInt

foreign import capi "poll.h value POLLIN" c_POLLIN :: CShort

pollIn :: Int16
pollIn = fromIntegral c_POLLIN

readInput :: Term -> P -> Int32 -> IO Int32
readInput t buf len = do
  deliverDeath t
  i <- readIORef (tInt t)
  if i && len >= 1
    then do
      writeIORef (tInt t) False
      wrW8 buf 0 3
      pure 1
    else do
      when i $ writeIORef (tInt t) False
      w <- readIORef (tWinch t)
      winch <-
        if w
          then do
            writeIORef (tWinch t) False
            s <- winSize t
            case s of
              Just (rows, cols) | len >= 32 -> do
                -- what the C formats with vim_snprintf
                let b = map (fromIntegral . ord) ("\ESC[48;" ++ show rows ++ ";" ++ show cols ++ ";0;0t")
                mapM_ (\(k, c) -> wrW8 buf k c) (zip [0 ..] b)
                wrW8 buf (length b) 0
                pure (Just (fromIntegral (length b)))
              _ -> pure Nothing
          else pure Nothing
      case winch of
        Just n -> pure n
        Nothing -> do
          s <- readIORef (tTstp t)
          if s
            then do
              writeIORef (tTstp t) False
              if len >= 5
                then do
                  mapM_ (\(k, c) -> wrW8 buf k c) (zip [0 ..] [27, 91, 63, 49, 122])
                  pure 5
                else readIn
            else readIn
  where
    readIn = do
      r <- try (fdReadBuf (tIn t) (castPtr buf) (fromIntegral len)) :: IO (Either IOException ByteCount)
      pure (either (const (-1)) fromIntegral r)

suspend :: IO ()
suspend = do
  void $ installHandler sigTSTP Default Nothing
  signalProcessGroup sigTSTP 0
  void $ installHandler sigTSTP onTstp Nothing

-- | msg's len bytes (to its NUL when len is negative), to the error stream
-- when err.
message :: Term -> P -> Int32 -> Bool -> IO ()
message t msg len err = do
  n <- if len < 0 then cLen else pure (fromIntegral len)
  let fd = if err then tErr t else tOut t
      go off = when (off < n) $ do
        r <- try (fdWriteBuf fd (castPtr (pAdd msg off)) (fromIntegral (n - off))) :: IO (Either IOException ByteCount)
        case r of
          Right w | w > 0 -> go (off + fromIntegral w)
          _ -> pure ()
  go 0
  where
    cLen = let go i = do c <- rdW8 msg i; if c == 0 then pure i else go (i + 1) in go 0
