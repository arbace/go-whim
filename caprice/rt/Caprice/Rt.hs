{-# LANGUAGE BangPatterns #-}

-- | The runtime the generated editor is written against: C's memory, kept
-- as C keeps it.  Every C object lives in raw memory laid out as the C lays
-- it out on amd64 -- the file-scope objects in one segment per editor, a
-- local whose address is taken or that is an array, a struct or a union in
-- its call's frame, an allocation in memory that is never freed (the C
-- host's arena never frees either) -- and a pointer is an address.  So the
-- translation reads and writes bytes at offsets, and C's pointer arithmetic,
-- comparison and punning are the machine's (doc/HASKELL.md).
module Caprice.Rt
  ( module Caprice.Rt
  , module Data.Int
  , module Data.Word
  , module Data.Bits
  , Ptr (..)
  , nullPtr
  , plusPtr
  , minusPtr
  , castPtr
  , Dynamic
  , toDyn
  , fromDyn
  ) where

import Control.Concurrent (forkIO, getNumCapabilities)
import Control.Concurrent.MVar (newEmptyMVar, putMVar, takeMVar)
import Control.Exception (SomeException, throwIO, try)
import Control.Monad (forM, forM_, unless)
import Data.Bits
import Data.Char (ord)
import Data.Dynamic (Dynamic, fromDyn, toDyn)
import Data.IORef (atomicWriteIORef, newIORef, readIORef)
import Data.Int
import Data.Word
import Foreign.Marshal.Alloc (allocaBytesAligned, callocBytes)
import Foreign.Marshal.Utils (copyBytes, fillBytes, moveBytes)
import Foreign.Ptr (castPtr, minusPtr, nullPtr, plusPtr, ptrToWordPtr, wordPtrToPtr)
import Foreign.Storable (peekByteOff, pokeByteOff)
import GHC.Ptr (Ptr (..))

-- | An address, of anything: C's pointers, all of one type.
type P = Ptr ()

-- | An editor: its data segment, where the C's file-scope objects are, and
-- its host -- what the host module made it with, as a Dynamic, since the
-- runtime does not know the host's types (Caprice.Host's EdHost; a test's
-- ()).  Several editors run at once in one process, each on its own.
data Ed = Ed {edSeg :: !P, edHost :: !Dynamic}

-- | An editor whose segment is n zeroed bytes, on host h.
newEd :: Int -> Dynamic -> IO Ed
newEd n h = do
  seg <- callocBytes (max n 16)
  pure (Ed seg h)

-- * Reading and writing memory: the address and a constant offset

rdI8 :: P -> Int -> IO Int8
rdI8 = peekByteOff
{-# INLINE rdI8 #-}
rdW8 :: P -> Int -> IO Word8
rdW8 = peekByteOff
{-# INLINE rdW8 #-}
rdI16 :: P -> Int -> IO Int16
rdI16 = peekByteOff
{-# INLINE rdI16 #-}
rdW16 :: P -> Int -> IO Word16
rdW16 = peekByteOff
{-# INLINE rdW16 #-}
rdI32 :: P -> Int -> IO Int32
rdI32 = peekByteOff
{-# INLINE rdI32 #-}
rdW32 :: P -> Int -> IO Word32
rdW32 = peekByteOff
{-# INLINE rdW32 #-}
rdI64 :: P -> Int -> IO Int64
rdI64 = peekByteOff
{-# INLINE rdI64 #-}
rdW64 :: P -> Int -> IO Word64
rdW64 = peekByteOff
{-# INLINE rdW64 #-}
rdP :: P -> Int -> IO P
rdP = peekByteOff
{-# INLINE rdP #-}

-- | A C bool is a byte, 0 or 1.
rdB :: P -> Int -> IO Bool
rdB p o = (/= (0 :: Word8)) <$> peekByteOff p o
{-# INLINE rdB #-}

wrI8 :: P -> Int -> Int8 -> IO ()
wrI8 = pokeByteOff
{-# INLINE wrI8 #-}
wrW8 :: P -> Int -> Word8 -> IO ()
wrW8 = pokeByteOff
{-# INLINE wrW8 #-}
wrI16 :: P -> Int -> Int16 -> IO ()
wrI16 = pokeByteOff
{-# INLINE wrI16 #-}
wrW16 :: P -> Int -> Word16 -> IO ()
wrW16 = pokeByteOff
{-# INLINE wrW16 #-}
wrI32 :: P -> Int -> Int32 -> IO ()
wrI32 = pokeByteOff
{-# INLINE wrI32 #-}
wrW32 :: P -> Int -> Word32 -> IO ()
wrW32 = pokeByteOff
{-# INLINE wrW32 #-}
wrI64 :: P -> Int -> Int64 -> IO ()
wrI64 = pokeByteOff
{-# INLINE wrI64 #-}
wrW64 :: P -> Int -> Word64 -> IO ()
wrW64 = pokeByteOff
{-# INLINE wrW64 #-}
wrP :: P -> Int -> P -> IO ()
wrP = pokeByteOff
{-# INLINE wrP #-}
wrB :: P -> Int -> Bool -> IO ()
wrB p o b = pokeByteOff p o (if b then 1 else 0 :: Word8)
{-# INLINE wrB #-}

-- | n bytes from src to dst, overlapping or not: a struct's copy.
copyMem :: P -> P -> Int -> IO ()
copyMem dst src n = moveBytes dst src n
{-# INLINE copyMem #-}

-- | n bytes at p, each c.
fillMem :: P -> Word8 -> Int -> IO ()
fillMem p c n = fillBytes p c n
{-# INLINE fillMem #-}

-- | n bytes from src to dst that do not overlap.
copyMemNo :: P -> P -> Int -> IO ()
copyMemNo dst src n = copyBytes dst src n
{-# INLINE copyMemNo #-}

-- * C's conversions that are not fromIntegral's

-- | A truth value as a number: 1 or 0.
b2i :: Num a => Bool -> a
b2i b = if b then 1 else 0

-- | A C character constant: 'x' as the integer of its code, of any type.
ch :: Num a => Char -> a
ch = fromIntegral . ord
{-# INLINE ch #-}
{-# INLINE b2i #-}

-- | A pointer's bits.
p2i :: P -> Word64
p2i = fromIntegral . ptrToWordPtr
{-# INLINE p2i #-}

-- | Bits as a pointer.
i2p :: Word64 -> P
i2p = wordPtrToPtr . fromIntegral
{-# INLINE i2p #-}

-- | p plus n bytes.
pAdd :: P -> Int -> P
pAdd = plusPtr
{-# INLINE pAdd #-}

-- | The bytes from q to p.
pSub :: P -> P -> Int
pSub = minusPtr
{-# INLINE pSub #-}

-- * A call's frame

-- | The locals of a call that live in memory: n bytes, zeroed, for as long
-- as the call runs.
frame :: Int -> (P -> IO a) -> IO a
frame n k = allocaBytesAligned n 16 $ \p -> fillBytes p 0 n >> k p
{-# INLINE frame #-}

-- * Function pointers

-- | A function's address: its index in the table, at an address no object
-- has, since a Haskell function cannot live in raw memory.
fnPtr :: Int -> P
fnPtr i = i2p (fromIntegral (i * 16 + 16))
{-# INLINE fnPtr #-}

-- | The index a function's address holds.
fnIndex :: P -> Int
fnIndex p = fromIntegral (p2i p `shiftR` 4) - 1
{-# INLINE fnIndex #-}

-- | A value as the 64 bits a call through a function pointer passes: every
-- argument and result goes through the table this way.
class Raw a where
  toRaw :: a -> Word64
  fromRaw :: Word64 -> a

instance Raw Int8 where
  toRaw = fromIntegral
  fromRaw = fromIntegral
instance Raw Word8 where
  toRaw = fromIntegral
  fromRaw = fromIntegral
instance Raw Int16 where
  toRaw = fromIntegral
  fromRaw = fromIntegral
instance Raw Word16 where
  toRaw = fromIntegral
  fromRaw = fromIntegral
instance Raw Int32 where
  toRaw = fromIntegral
  fromRaw = fromIntegral
instance Raw Word32 where
  toRaw = fromIntegral
  fromRaw = fromIntegral
instance Raw Int64 where
  toRaw = fromIntegral
  fromRaw = fromIntegral
instance Raw Word64 where
  toRaw = id
  fromRaw = id
instance Raw Bool where
  toRaw = b2i
  fromRaw = (/= 0)
instance Raw () where
  toRaw _ = 0
  fromRaw _ = ()
instance Raw (Ptr a) where
  toRaw = fromIntegral . ptrToWordPtr
  fromRaw = wordPtrToPtr . fromIntegral

-- | An argument of a variadic call, as C's default promotions leave it: a
-- signed integer, an unsigned one, or a pointer.
data VArg = VI !Int64 | VU !Word64 | VP !P

-- | The C string at p, as bytes.
cBytes :: P -> IO [Word8]
cBytes p = go 0
  where
    go !i = do
      c <- rdW8 p i
      if c == 0 then pure [] else (c :) <$> go (i + 1)

-- * A loop over lines, in parallel

-- | work over [0, n) in chunks at once, and whether every chunk's work did:
-- the parallel body of a function the C writes as one loop over a range
-- (match_lines, as editor/chunks.go's Chunks and braaam's Rt.chunks).  About
-- four chunks a capability, none smaller than 64; a range of one chunk runs
-- on the caller's thread, and a chunk that did not stops the chunks not yet
-- started.  The work must be the loop's over its part and write nothing
-- another part reads.
chunks :: Int -> (Int -> Int -> IO Bool) -> IO Bool
chunks n work = do
  caps <- getNumCapabilities
  let w = 4 * caps
      size = max 64 ((n + w - 1) `div` w)
  if size >= n
    then work 0 n
    else do
      failed <- newIORef False
      dones <- forM [0, size .. n - 1] $ \from -> do
        done <- newEmptyMVar
        _ <- forkIO $ do
          r <- try $ do
            f <- readIORef failed
            ok <- if f then pure False else work from (min n (from + size))
            unless ok $ atomicWriteIORef failed True
          putMVar done (r :: Either SomeException ())
        pure done
      rs <- mapM takeMVar dones
      forM_ rs $ either throwIO pure
      not <$> readIORef failed
