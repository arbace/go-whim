-- | Editors are instances: four run at once in one process, each on a host
-- of this program's own -- keys from an IORef, the screen into another, a
-- fixed size -- and none sees another's text.  editor/host_test.go's
-- TestEditorsAreInstances, in Haskell; caprice_test.go runs it; it is a Main of its own, not -main-is, which would change GHC's flags and recompile the core.
module Main (main) where

import Caprice.Host (Host (..))
import Caprice.Rt
import qualified Caprice.Run as Run
import Control.Concurrent (forkIO)
import Control.Concurrent.MVar (newEmptyMVar, putMVar, takeMVar)
import Control.Monad (forM, forM_, when)
import qualified Data.ByteString as B
import qualified Data.ByteString.Char8 as BC
import qualified Data.ByteString.Unsafe as BU
import Data.IORef
import Foreign.Marshal.Utils (copyBytes)
import System.Exit (exitFailure)

-- | A host with no operating system under it: what an embedding program
-- would write.
fakeHost :: IORef B.ByteString -> IORef [B.ByteString] -> IORef [B.ByteString] -> Host
fakeHost keys screen msgs =
  Host
    { hInit = \_ -> pure ()
    , hWinSize = pure (Just (24, 80))
    , hTermStart = pure ()
    , hTermStop = pure ()
    , hTtyKeys = \_ -> pure (Just (0x7f, 3, True, True))
    , hNowMs = pure 0
    , hTime = pure 0
    , hDelay = \_ _ -> pure ()
    , hWaitForInput = \_ -> not . B.null <$> readIORef keys
    , hReadInput = \buf len -> do
        k <- readIORef keys
        let (now, rest) = B.splitAt (fromIntegral len) k
        writeIORef keys rest
        BU.unsafeUseAsCStringLen now $ \(p, n) -> copyBytes (castPtr buf) p n
        pure (fromIntegral (B.length now))
    , hRaise = \_ -> pure ()
    , hSuspend = pure ()
    , hMessage = \m len _ -> bytes m len >>= \b -> modifyIORef' msgs (b :)
    , hWrite = \s len -> bytes s len >>= \b -> modifyIORef' screen (b :) >> pure len
    }
  where
    bytes p len
      | len >= 0 = B.packCStringLen (castPtr p, fromIntegral len)
      | otherwise = B.packCString (castPtr p)

main :: IO ()
main = do
  let n = 4 :: Int
      text i = BC.pack ("editing number " ++ show i)
  runs <- forM [0 .. n - 1] $ \i -> do
    keys <- newIORef (B.concat [BC.pack "i", text i, BC.pack "\ESC:q!\r"])
    screen <- newIORef []
    msgs <- newIORef []
    done <- newEmptyMVar
    _ <- forkIO (Run.run (fakeHost keys screen msgs) [BC.pack "caprice"] >>= putMVar done)
    pure (screen, msgs, done)
  bad <- newIORef False
  forM_ (zip [0 ..] runs) $ \(i, (screen, msgs, done)) -> do
    status <- takeMVar done
    s <- B.concat . reverse <$> readIORef screen
    when (status /= 0) $ do
      m <- B.concat . reverse <$> readIORef msgs
      putStrLn ("editor " ++ show i ++ ": status " ++ show status ++ "; messages " ++ show m)
      writeIORef bad True
    forM_ [0 .. n - 1] $ \j -> do
      let has = not (B.null (snd (B.breakSubstring (text j) s)))
      when (has /= (i == j)) $ do
        putStrLn ("editor " ++ show i ++ "'s screen shows editor " ++ show j ++ "'s text: " ++ show has)
        writeIORef bad True
  b <- readIORef bad
  if b then exitFailure else putStrLn ("ok: " ++ show n ++ " editors, each its own")
