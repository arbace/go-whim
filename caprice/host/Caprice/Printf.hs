{-# LANGUAGE BangPatterns #-}

-- | vim's own printf, vim_snprintf, from the host half of whim-vim.c: the
-- core calls it for every message it formats.  A port of editor/format.go
-- (itself the C's port) onto the C's memory: the format and the result are
-- bytes at addresses.  The C's va_list is the call's arguments and a cursor
-- into them; va_copy(ap, ap_start) sets the cursor back to 0.
--
-- vim_vsnprintf_typval is only ever called with tvs == NULL (by
-- vim_vsnprintf), so every `tvs != nullptr` is false: get_unsigned_int clamps
-- instead of reporting an overflow, and the "too many arguments" check never
-- runs.
module Caprice.Printf (vim_snprintf) where

import Caprice.Rt
import {-# SOURCE #-} Caprice.Editor (addr'IObuff, emsg, emsg_iobuff_room, iemsg, iobuff_or, utf_ptr2cells, utfc_ptr2len)
import Control.Monad (forM_, when)
import Data.Char (ord)
import Data.IORef
import Foreign.Marshal.Alloc (allocaBytes, callocBytes)
import GHC.Ptr (Ptr (..))

tmpLen :: Int
tmpLen = 350

maxWidth :: Word32
maxWidth = 1048576

-- the C's type codes
tUnknown, tInt, tLongInt, tLongLongInt, tUnsignedInt, tUnsignedLongInt, tUnsignedLongLongInt, tPointer, tPercent, tChar, tString :: Int
tUnknown = -1
tInt = 0
tLongInt = 1
tLongLongInt = 2
tUnsignedInt = 3
tUnsignedLongInt = 4
tUnsignedLongLongInt = 5
tPointer = 6
tPercent = 7
tChar = 8
tString = 9

-- | A C string of the host's, NUL-terminated, in static memory.
lit :: String -> IO P
lit s = do
  p <- callocBytes (length s + 1)
  forM_ (zip [0 ..] s) $ \(i, c) -> wrW8 p i (fromIntegral (ord c))
  pure p

-- | The byte at p + i.
at :: P -> Int -> IO Word8
at p i = rdW8 p i
{-# INLINE at #-}

isDigit8 :: Word8 -> Bool
isDigit8 c = c >= 48 && c <= 57

cStrlen :: P -> IO Int
cStrlen p = go 0
  where
    go !i = do
      c <- at p i
      if c == 0 then pure i else go (i + 1)

-- | strchr(s, c) as an offset from s, or -1.
cStrchr :: P -> Word8 -> IO Int
cStrchr s c = go 0
  where
    go !i = do
      x <- at s i
      if x == c then pure i else if x == 0 then pure (-1) else go (i + 1)

-- * The va_list

data Va = Va {vaArgs :: [VArg], vaNext :: IORef Int}

vaNextArg :: Va -> IO VArg
vaNextArg (Va as r) = do
  i <- readIORef r
  writeIORef r (i + 1)
  case drop i as of
    (a : _) -> pure a
    [] -> error "vim_snprintf: va_arg past the last argument"

vaBits :: VArg -> Int64
vaBits (VI v) = v
vaBits (VU v) = fromIntegral v
vaBits (VP p) = fromIntegral (p2i p)

vaInt :: Va -> IO Int32
vaInt ap = fromIntegral . vaBits <$> vaNextArg ap

vaUint :: Va -> IO Word32
vaUint ap = fromIntegral . vaBits <$> vaNextArg ap

vaLong :: Va -> IO Int64
vaLong ap = vaBits <$> vaNextArg ap

vaUlong :: Va -> IO Word64
vaUlong ap = fromIntegral . vaBits <$> vaNextArg ap

vaPtr :: Va -> IO P
vaPtr ap = do
  a <- vaNextArg ap
  pure $ case a of
    VP p -> p
    v -> i2p (fromIntegral (vaBits v))

-- * The C host's helpers

fmtbase :: Word8 -> Word64
fmtbase c
  | c == ord8 'o' = 8
  | c == ord8 'x' || c == ord8 'X' = 16
  | otherwise = 10

ord8 :: Char -> Word8
ord8 = fromIntegral . ord

-- | The digits of v at dest, its sign first when isneg; how many.
fmtnum :: P -> Int -> Word64 -> Word64 -> Bool -> Bool -> IO Int
fmtnum dest off v0 base upper isneg = do
  let v = if isneg then complement v0 + 1 else v0
      digits = go v []
      go x acc =
        let d = x `rem` base
            c
              | d < 10 = 48 + fromIntegral d
              | upper = 65 + fromIntegral d - 10
              | otherwise = 97 + fromIntegral d - 10
            acc' = (c :: Word8) : acc
         in if x `quot` base == 0 then acc' else go (x `quot` base) acc'
      out = (if isneg then [45] else []) ++ digits
  forM_ (zip [0 ..] out) $ \(i, c) -> wrW8 dest (off + i) c
  wrW8 dest (off + length out) 0
  pure (length out)

fmtptr :: P -> Int -> Word64 -> IO Int
fmtptr dest off v = do
  wrW8 dest off 48
  wrW8 dest (off + 1) 120
  forM_ [0 .. 15] $ \i -> do
    let d = fromIntegral ((v `shiftR` (60 - 4 * i)) .&. 0xf) :: Word8
    wrW8 dest (off + 2 + i) (if d < 10 then 48 + d else 97 + d - 10)
  wrW8 dest (off + 18) 0
  pure 18

-- | The type a format's conversion reads.
formatTypeof :: P -> IO Int
formatTypeof t0 = do
  c0 <- at t0 0
  (lm, t) <-
    if c0 == ord8 'h' || c0 == ord8 'l'
      then do
        c1 <- at t0 1
        if c0 == ord8 'l' && c1 == ord8 'l' then pure (ord8 'L', 2) else pure (c0, 1)
      else pure (0, 0)
  spec0 <- at t0 t
  let (spec, lm')
        | spec0 == ord8 'i' = (ord8 'd', lm)
        | spec0 == ord8 '*' = (ord8 'd', ord8 'h')
        | spec0 == ord8 'D' = (ord8 'd', ord8 'l')
        | spec0 == ord8 'U' = (ord8 'u', ord8 'l')
        | spec0 == ord8 'O' = (ord8 'o', ord8 'l')
        | otherwise = (spec0, lm)
      byLm s l u ll = if lm' == 0 || lm' == ord8 'h' then s else if lm' == ord8 'l' then l else if lm' == ord8 'L' then ll else u
  pure $ case toEnum (fromIntegral spec) :: Char of
    '%' -> tPercent
    'c' -> tChar
    's' -> tString
    'S' -> tString
    'p' -> tPointer
    'b' -> tUnsignedLongLongInt
    'B' -> tUnsignedLongLongInt
    'd' -> byLm tInt tLongInt tUnknown tLongLongInt
    x | x `elem` "uoxX" -> byLm tUnsignedInt tUnsignedLongInt tUnknown tUnsignedLongLongInt
    _ -> tUnknown

formatTypename :: P -> IO P
formatTypename t = do
  k <- formatTypeof t
  lit $ case k of
    _ | k == tInt -> "int"
      | k == tLongInt -> "long int"
      | k == tLongLongInt -> "long long int"
      | k == tUnsignedInt -> "unsigned int"
      | k == tUnsignedLongInt -> "unsigned long int"
      | k == tUnsignedLongLongInt -> "unsigned long long int"
      | k == tPointer -> "pointer"
      | k == tPercent -> "percent"
      | k == tChar -> "char"
      | k == tString -> "string"
      | otherwise -> "unknown"

-- | The vim_snprintf-into-IObuff-then-emsg pair every format error makes.
fmtError :: Ed -> String -> [VArg] -> IO ()
fmtError ed msg args = do
  m <- lit msg
  iob <- rdP (addr'IObuff ed) 0
  room <- emsg_iobuff_room ed
  _ <- vim_snprintf ed iob room m args
  s <- iobuff_or ed m
  _ <- emsg ed s
  pure ()

eMix, eUnused, eReused, eInconsistent, eInvalid :: String
eMix = "E1500: Cannot mix positional and non-positional arguments: %s"
eUnused = "E1501: format argument %d unused in $-style format: %s"
eReused = "E1502: Positional argument %d used as field width reused as different type: %s/%s"
eInconsistent = "E1504: Positional argument %d type used inconsistently: %s/%s"
eInvalid = "E1505: Invalid format specifier: %s"

-- | ap_types: a growable table of pointers into the format, nullPtr for none.
type Types = IORef [P]

adjustTypes :: Ed -> Types -> Int -> P -> IO Bool
adjustTypes ed apTypes arg t
  | arg <= 0 = fmtError ed eInvalid [VP t] >> pure False
  | otherwise = do
      ts <- readIORef apTypes
      let ts' = if length ts < arg then ts ++ replicate (arg - length ts) nullPtr else ts
      writeIORef apTypes ts'
      let cur = ts' !! (arg - 1)
      ok <-
        if cur == nullPtr
          then pure True
          else do
            c0 <- at cur 0
            t0 <- at t 0
            if c0 == ord8 '*' || t0 == ord8 '*'
              then do
                let pt = if t0 == ord8 '*' then cur else t
                p0 <- at pt 0
                if p0 /= ord8 '*' && p0 /= ord8 'd' && p0 /= ord8 'i'
                  then do
                    a <- formatTypename cur
                    b <- formatTypename t
                    fmtError ed eReused [VI (fromIntegral arg), VP a, VP b]
                    pure False
                  else pure True
              else do
                k1 <- formatTypeof t
                k2 <- formatTypeof cur
                if k1 /= k2
                  then do
                    a <- formatTypename t
                    b <- formatTypename cur
                    fmtError ed eInconsistent [VI (fromIntegral arg), VP a, VP b]
                    pure False
                  else pure True
      when ok $ modifyIORef' apTypes (\xs -> take (arg - 1) xs ++ [t] ++ drop arg xs)
      pure ok

-- | The digits at *p: their value, clamped; p after them.
getUnsigned :: IORef P -> IO Word32
getUnsigned pr = do
  p <- readIORef pr
  c <- at p 0
  let go !q !uj = do
        x <- at q 0
        if isDigit8 x && uj < maxWidth then go (pAdd q 1) (10 * uj + fromIntegral (x - 48)) else pure (q, uj)
  (q, uj) <- go (pAdd p 1) (fromIntegral c - 48)
  writeIORef pr q
  pure (min uj maxWidth)

-- | Whether a format's positional arguments are consistent, and their types.
parseFmtTypes :: Ed -> Types -> P -> IO Bool
parseFmtTypes ed apTypes fmt
  | fmt == nullPtr = pure True
  | otherwise = do
      anyPos <- newIORef False
      anyArg <- newIORef False
      pr <- newIORef fmt
      let failed = writeIORef apTypes [] >> pure False
          mixed = do
            a <- readIORef anyPos
            b <- readIORef anyArg
            if a && b then fmtError ed eMix [VP fmt] >> pure True else pure False
          loop = do
            p <- readIORef pr
            c <- at p 0
            if c == 0
              then pure True
              else
                if c /= ord8 '%'
                  then do
                    q <- cStrchr (pAdd p 1) 37
                    n <- if q < 0 then cStrlen p else pure (q + 1)
                    writeIORef pr (pAdd p n)
                    loop
                  else do
                    r <- conversion
                    if r then loop else pure False
          conversion = do
            p0 <- readIORef pr
            let pstart = pAdd p0 1
            writeIORef pr pstart
            let skipDigits q = do x <- at q 0; if isDigit8 x then skipDigits (pAdd q 1) else pure q
            ptype0 <- skipDigits pstart
            d <- at ptype0 0
            posArg <-
              if d == ord8 '$'
                then do
                  p <- readIORef pr
                  z <- at p 0
                  if z == ord8 '0'
                    then fmtError ed eInvalid [VP fmt] >> pure Nothing
                    else do
                      uj <- getUnsigned pr
                      writeIORef anyPos True
                      m <- mixed
                      if m then pure Nothing else do modifyIORef' pr (`pAdd` 1); pure (Just (Just (fromIntegral uj)))
                else pure (Just Nothing)
            case posArg of
              Nothing -> failed
              Just pa -> do
                let skipFlags = do
                      p <- readIORef pr
                      x <- at p 0
                      when (x `elem` map ord8 "0-+ #'") $ writeIORef pr (pAdd p 1) >> skipFlags
                skipFlags
                ok1 <- starOrDigits False
                if not ok1
                  then failed
                  else do
                    p <- readIORef pr
                    x <- at p 0
                    ok2 <- if x == ord8 '.' then writeIORef pr (pAdd p 1) >> starOrDigits True else pure True
                    if not ok2
                      then failed
                      else do
                        ptype <- case pa of
                          Just _ -> do
                            writeIORef anyPos True
                            readIORef pr
                          Nothing -> pure ptype0
                        m <- case pa of
                          Just _ -> mixed
                          Nothing -> pure False
                        if m
                          then failed
                          else do
                            q <- readIORef pr
                            l <- at q 0
                            when (l == ord8 'h' || l == ord8 'l') $ do
                              l2 <- at q 1
                              writeIORef pr (if l == ord8 'l' && l2 == ord8 'l' then pAdd q 2 else pAdd q 1)
                            q' <- readIORef pr
                            spec <- at q' 0
                            ok3 <-
                              if spec `elem` map ord8 "i*duoDUOxXbBcsSp"
                                then case pa of
                                  Just a -> adjustTypes ed apTypes a ptype
                                  Nothing -> do
                                    writeIORef anyArg True
                                    not <$> mixed
                                else case pa of
                                  Just _ -> fmtError ed eMix [VP fmt] >> pure False
                                  Nothing -> pure True
                            if not ok3
                              then failed
                              else do
                                q2 <- readIORef pr
                                s <- at q2 0
                                when (s /= 0) $ writeIORef pr (pAdd q2 1)
                                pure True
          -- a field width (prec False) or a precision (True): *, *N$, or digits
          starOrDigits prec = do
            arg <- readIORef pr
            x <- at arg 0
            if x == ord8 '*'
              then do
                writeIORef pr (pAdd arg 1)
                p <- readIORef pr
                y <- at p 0
                if isDigit8 y
                  then do
                    uj <- getUnsigned pr
                    q <- readIORef pr
                    z <- at q 0
                    if z /= ord8 '$'
                      then fmtError ed eInvalid [VP fmt] >> pure False
                      else do
                        writeIORef pr (pAdd q 1)
                        writeIORef anyPos True
                        m <- mixed
                        if m then pure False else adjustTypes ed apTypes (fromIntegral uj) arg
                  else do
                    writeIORef anyArg True
                    not <$> mixed
              else
                if isDigit8 x
                  then do
                    _ <- getUnsigned pr
                    q <- readIORef pr
                    z <- at q 0
                    if z == ord8 '$' then fmtError ed eInvalid [VP fmt] >> pure False else pure True
                  else pure True
      ok <- loop
      if not ok
        then pure False
        else do
          ts <- readIORef apTypes
          let unused = [i | (i, t) <- zip [0 :: Int ..] ts, t == nullPtr]
          case unused of
            (i : _) -> do
              fmtError ed eUnused [VI (fromIntegral (i + 1)), VP fmt]
              writeIORef apTypes []
              pure False
            [] -> pure True

-- | The argument the next conversion reads: va_arg to it.
skipToArg :: Ed -> [P] -> Va -> IORef Int -> IORef Int -> P -> IO ()
skipToArg ed apTypes ap argIdxR argCurR fmt = do
  argIdx <- readIORef argIdxR
  argCur <- readIORef argCurR
  if argCur + 1 == argIdx
    then writeIORef argCurR (argCur + 1) >> writeIORef argIdxR (argIdx + 1)
    else do
      argMin <-
        if argCur >= argIdx
          then writeIORef (vaNext ap) 0 >> pure 0
          else pure argCur
      let go i
            | i >= argIdx - 1 = pure (Right i)
            | i >= length apTypes || apTypes !! i == nullPtr = do
                m <- lit "E1507: Internal error: ap_types or ap_types[idx] is NULL: %d: %s"
                iob <- rdP (addr'IObuff ed) 0
                room <- emsg_iobuff_room ed
                _ <- vim_snprintf ed iob room m [VI (fromIntegral i), VP fmt]
                s <- iobuff_or ed m
                iemsg ed s
                pure (Left i)
            | otherwise = do
                k <- formatTypeof (apTypes !! i)
                when (k /= tPercent && k /= tUnknown) $ () <$ vaNextArg ap
                go (i + 1)
      r <- go argMin
      case r of
        Left i -> writeIORef argCurR i -- the C returns with the cursor where it stopped
        Right i -> writeIORef argCurR (i + 1) >> writeIORef argIdxR (argIdx + 1)

-- | vim_snprintf(str, str_m, fmt, ...): the result's length, what it would
-- have been had str_m been large enough.
vim_snprintf :: Ed -> P -> Word64 -> P -> [VArg] -> IO Int32
vim_snprintf ed str strM fmt args = do
  typesR <- newIORef []
  ok <- parseFmtTypes ed typesR fmt
  if not ok
    then pure 0
    else do
      apTypes <- readIORef typesR
      nextR <- newIORef 0
      let ap = Va args nextR
      strL <- newIORef (0 :: Word64)
      argCur <- newIORef 0
      argIdx <- newIORef 1
      pr <- newIORef (if fmt == nullPtr then nullPtr else fmt)
      let emit src n = do
            l <- readIORef strL
            when (l < strM) $ copyMem (pAdd str (fromIntegral l)) src (fromIntegral (min n (strM - l)))
            writeIORef strL (l + n)
          pad c n = do
            l <- readIORef strL
            when (l < strM) $ fillMem (pAdd str (fromIntegral l)) c (fromIntegral (min n (strM - l)))
            writeIORef strL (l + n)
          loop = do
            p <- readIORef pr
            c <- if p == nullPtr then pure 0 else at p 0
            when (c /= 0) $
              if c /= ord8 '%'
                then do
                  q <- cStrchr (pAdd p 1) 37
                  n <- if q < 0 then cStrlen p else pure (q + 1)
                  emit p (fromIntegral n)
                  writeIORef pr (pAdd p n)
                  loop
                else conversion >> loop
          conversion = allocaBytes tmpLen $ \tmp -> allocaBytes 2 $ \uchar -> do
            modifyIORef' pr (`pAdd` 1)
            p0 <- readIORef pr
            let skipDigits q = do x <- at q 0; if isDigit8 x then skipDigits (pAdd q 1) else pure q
            pt <- skipDigits p0
            d <- at pt 0
            posArg <-
              if d == ord8 '$'
                then do
                  uj <- getUnsigned pr
                  modifyIORef' pr (`pAdd` 1)
                  pure (fromIntegral uj :: Int)
                else pure (-1)
            zeroPad <- newIORef False
            justifyLeft <- newIORef False
            forceSign <- newIORef False
            spaceForPositive <- newIORef True
            alternate <- newIORef False
            let flags = do
                  p <- readIORef pr
                  x <- at p 0
                  let set r v = writeIORef r v >> writeIORef pr (pAdd p 1) >> flags
                  case toEnum (fromIntegral x) :: Char of
                    '0' -> set zeroPad True
                    '-' -> set justifyLeft True
                    '+' -> writeIORef spaceForPositive False >> set forceSign True
                    ' ' -> set forceSign True
                    '#' -> set alternate True
                    '\'' -> writeIORef pr (pAdd p 1) >> flags
                    _ -> pure ()
            flags
            minWidth <- newIORef (0 :: Word64)
            precision <- newIORef (0 :: Word64)
            precSpecified <- newIORef False
            let starArg = do
                  p <- readIORef pr
                  x <- at p 0
                  when (isDigit8 x) $ do
                    uj <- getUnsigned pr
                    writeIORef argIdx (fromIntegral uj)
                    modifyIORef' pr (`pAdd` 1)
                  skipToArg ed apTypes ap argIdx argCur fmt
                  j <- vaInt ap
                  pure (min j (fromIntegral maxWidth))
            p1 <- readIORef pr
            w <- at p1 0
            if w == ord8 '*'
              then do
                writeIORef pr (pAdd p1 1)
                j <- starArg
                if j >= 0
                  then writeIORef minWidth (fromIntegral j)
                  else writeIORef minWidth (fromIntegral (negate j)) >> writeIORef justifyLeft True
              else when (isDigit8 w) $ getUnsigned pr >>= writeIORef minWidth . fromIntegral
            p2 <- readIORef pr
            dot <- at p2 0
            when (dot == ord8 '.') $ do
              writeIORef pr (pAdd p2 1)
              writeIORef precSpecified True
              p3 <- readIORef pr
              x <- at p3 0
              if isDigit8 x
                then getUnsigned pr >>= writeIORef precision . fromIntegral
                else when (x == ord8 '*') $ do
                  writeIORef pr (pAdd p3 1)
                  j <- starArg
                  if j >= 0 then writeIORef precision (fromIntegral j) else writeIORef precSpecified False >> writeIORef precision 0
            p4 <- readIORef pr
            l0 <- at p4 0
            lm <-
              if l0 == ord8 'h' || l0 == ord8 'l'
                then do
                  l1 <- at p4 1
                  if l0 == ord8 'l' && l1 == ord8 'l'
                    then writeIORef pr (pAdd p4 2) >> pure (ord8 'L')
                    else writeIORef pr (pAdd p4 1) >> pure l0
                else pure 0
            pS <- readIORef pr
            spec0 <- at pS 0
            let (spec, lm')
                  | spec0 == ord8 'i' = (ord8 'd', lm)
                  | spec0 == ord8 'D' = (ord8 'd', ord8 'l')
                  | spec0 == ord8 'U' = (ord8 'u', ord8 'l')
                  | spec0 == ord8 'O' = (ord8 'o', ord8 'l')
                  | otherwise = (spec0, lm)
            when (posArg /= -1) $ writeIORef argIdx posArg
            strArg <- newIORef nullPtr
            strArgL <- newIORef (0 :: Word64)
            zerosToPad <- newIORef (0 :: Word64)
            zeroInd <- newIORef (0 :: Word64)
            let sc = toEnum (fromIntegral spec) :: Char
            if sc `elem` "%csS"
              then do
                writeIORef strArgL 1
                case sc of
                  '%' -> writeIORef strArg pS
                  'c' -> do
                    skipToArg ed apTypes ap argIdx argCur fmt
                    j <- vaInt ap
                    wrW8 uchar 0 (fromIntegral j)
                    writeIORef strArg uchar
                  _ -> do
                    skipToArg ed apTypes ap argIdx argCur fmt
                    s <- vaPtr ap
                    ps <- readIORef precSpecified
                    pc <- readIORef precision
                    if s == nullPtr
                      then do
                        nul <- lit "[NULL]"
                        writeIORef strArg nul
                        writeIORef strArgL 6
                      else do
                        writeIORef strArg s
                        if not ps
                          then cStrlen s >>= writeIORef strArgL . fromIntegral
                          else
                            if pc == 0
                              then writeIORef strArgL 0
                              else do
                                let lim = min pc 0x7fffffff
                                    find i
                                      | i >= lim = pure pc
                                      | otherwise = do x <- at s (fromIntegral i); if x == 0 then pure i else find (i + 1)
                                find 0 >>= writeIORef strArgL
                    when (sc == 'S') $ do
                      s' <- readIORef strArg
                      let cells !q !i = do
                            x <- at q 0
                            if x == 0
                              then pure (q, i)
                              else do
                                cell <- fromIntegral <$> utf_ptr2cells ed q
                                if ps && i + cell > pc
                                  then pure (q, i)
                                  else do
                                    n <- fromIntegral <$> utfc_ptr2len ed q
                                    cells (pAdd q n) (i + cell)
                      (q, i) <- cells s' 0
                      let l = fromIntegral (pSub q s')
                      writeIORef strArgL l
                      mw <- readIORef minWidth
                      when (mw /= 0) $ writeIORef minWidth (mw + l - i)
              else
                if sc `elem` "dubBoxXp"
                  then do
                    -- the argument, read as its conversion names it, and its sign
                    (v, neg, sign) <- case sc of
                      'p' -> do
                        skipToArg ed apTypes ap argIdx argCur fmt
                        pv <- p2i <$> vaPtr ap
                        pure (pv, False, if pv /= 0 then 1 else 0 :: Int)
                      _
                        | sc == 'b' || sc == 'B' -> do
                            skipToArg ed apTypes ap argIdx argCur fmt
                            b <- vaUlong ap
                            pure (b, False, if b /= 0 then 1 else 0)
                        | sc == 'd' -> do
                            skipToArg ed apTypes ap argIdx argCur fmt
                            -- the digits of the value at the conversion's width, the
                            -- sign of the value read (the C's)
                            (x, full) <-
                              if lm' == 0 || lm' == ord8 'h'
                                then do
                                  i <- vaInt ap
                                  pure (if lm' == ord8 'h' then fromIntegral (fromIntegral i :: Int16) else fromIntegral i :: Int64, fromIntegral i)
                                else do
                                  l <- vaLong ap
                                  pure (l, l)
                            pure (fromIntegral x, x < 0, if full > 0 then 1 else if full < 0 then -1 else 0)
                        | otherwise -> do
                            skipToArg ed apTypes ap argIdx argCur fmt
                            (x, full) <-
                              if lm' == 0 || lm' == ord8 'h'
                                then do
                                  u <- vaUint ap
                                  pure (if lm' == ord8 'h' then fromIntegral (fromIntegral u :: Word16) else fromIntegral u :: Word64, fromIntegral u)
                                else do
                                  u <- vaUlong ap
                                  pure (u, u)
                            pure (x, False, if full /= 0 then 1 else 0)
                    writeIORef strArg tmp
                    ps <- readIORef precSpecified
                    when ps $ writeIORef zeroPad False
                    fs <- readIORef forceSign
                    sfp <- readIORef spaceForPositive
                    alt <- readIORef alternate
                    lenR <- newIORef (0 :: Int)
                    let put c = do l <- readIORef lenR; wrW8 tmp l c; writeIORef lenR (l + 1)
                    if sc == 'd'
                      then when (fs && sign >= 0) $ put (if sfp then 32 else 43)
                      else when (alt && sign /= 0 && sc `elem` "bBxX") $ put 48 >> put spec
                    zi0 <- readIORef lenR
                    writeIORef zeroInd (fromIntegral zi0)
                    unless' ps $ writeIORef precision 1
                    pc <- readIORef precision
                    if pc == 0 && sign == 0
                      then pure ()
                      else do
                        l <- readIORef lenR
                        n <- case sc of
                          'p' -> fmtptr tmp l v
                          _
                            | sc == 'b' || sc == 'B' -> do
                                let bits x acc = let acc' = (48 + fromIntegral (x .&. 1)) : acc in if x `shiftR` 1 == 0 then acc' else bits (x `shiftR` 1) acc'
                                    bs = bits v []
                                forM_ (zip [0 ..] bs) $ \(i, c) -> wrW8 tmp (l + i) c
                                pure (length bs)
                            | sc == 'd' -> fmtnum tmp l v 10 False neg
                            | otherwise -> fmtnum tmp l v (fmtbase spec) (sc == 'X') False
                        writeIORef lenR (l + n)
                        l' <- readIORef lenR
                        zi <- readIORef zeroInd
                        c1 <- if fromIntegral zi < l' then at tmp (fromIntegral zi) else pure 0
                        when (fromIntegral zi < l' && c1 == 45) $ writeIORef zeroInd (zi + 1)
                        zi' <- readIORef zeroInd
                        when (fromIntegral zi' + 1 < l') $ do
                          a <- at tmp (fromIntegral zi')
                          b <- at tmp (fromIntegral zi' + 1)
                          when (a == 48 && (b == 120 || b == 88)) $ writeIORef zeroInd (zi' + 2)
                    l2 <- fromIntegral <$> readIORef lenR
                    writeIORef strArgL l2
                    zi2 <- readIORef zeroInd
                    let digits = l2 - zi2
                    zc <- if zi2 < l2 then at tmp (fromIntegral zi2) else pure 0
                    when (alt && sc == 'o' && not (zi2 < l2 && zc == 48)) $ do
                      pc' <- readIORef precision
                      when (not ps || pc' < digits + 1) $ writeIORef precision (digits + 1)
                    pc2 <- readIORef precision
                    when (digits < pc2) $ writeIORef zerosToPad (pc2 - digits)
                    jl <- readIORef justifyLeft
                    zp <- readIORef zeroPad
                    when (not jl && zp) $ do
                      mw <- readIORef minWidth
                      z <- readIORef zerosToPad
                      let n = fromIntegral (mw - (l2 + z)) :: Int32
                      when (n > 0) $ writeIORef zerosToPad (z + fromIntegral n)
                  else do
                    writeIORef zeroPad False
                    writeIORef justifyLeft True
                    writeIORef minWidth 0
                    writeIORef strArg pS
                    writeIORef strArgL (if spec /= 0 then 1 else 0)
            when (spec /= 0) $ writeIORef pr (pAdd pS 1)
            when (spec == 0) $ writeIORef pr pS
            jl <- readIORef justifyLeft
            mw <- readIORef minWidth
            sal <- readIORef strArgL
            z <- readIORef zerosToPad
            zp <- readIORef zeroPad
            let padN = fromIntegral (mw - (sal + z)) :: Int32
            when (not jl && padN > 0) $ pad (if zp then 48 else 32) (fromIntegral padN)
            sa <- readIORef strArg
            zi <- readIORef zeroInd
            if z == 0
              then writeIORef zeroInd 0
              else do
                let zn = fromIntegral zi :: Int32
                when (zn > 0) $ emit sa (fromIntegral zn)
                when ((fromIntegral z :: Int32) > 0) $ pad 48 z
            zi' <- readIORef zeroInd
            let sn = fromIntegral (sal - zi') :: Int32
            when (sn > 0) $ emit (pAdd sa (fromIntegral zi')) (fromIntegral sn)
            when (jl && padN > 0) $ pad 32 (fromIntegral padN)
      loop
      l <- readIORef strL
      when (strM > 0) $ wrW8 str (fromIntegral (if l <= strM - 1 then l else strM - 1)) 0
      pure (fromIntegral l)
  where
    unless' b act = if b then pure () else act
