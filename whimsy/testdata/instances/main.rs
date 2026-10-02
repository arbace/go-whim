//! Editors are instances: four at once in one process, each on a thread and
//! a host of its own -- keys from a buffer, the screen into a buffer, a
//! terminal of 24 by 80 -- each required to exit 0 with its own text on its
//! screen and no other's (editor/host_test.go's TestEditorsAreInstances,
//! caprice's testdata/instances).  whimsy_test.go compiles it against the
//! library `make bin/whimsy` leaves in lib/whimsy.

use std::cell::RefCell;
use std::rc::Rc;
use whimsy::host::{run, Host};

/// A host of the test's own: no terminal, no signals, no clock but a still
/// one.
struct Buffers {
    keys: RefCell<Vec<u8>>,
    screen: Rc<RefCell<Vec<u8>>>,
}

impl Host for Buffers {
    fn init(&self, _deathtrap: Rc<dyn Fn(i32)>) {}
    fn win_size(&self) -> Option<(i32, i32)> {
        Some((24, 80))
    }
    fn term_start(&self) {}
    fn term_stop(&self) {}
    fn tty_keys(&self, _fd: i32) -> Option<(i32, i32, bool, bool)> {
        None
    }
    fn now_ms(&self) -> i64 {
        0
    }
    fn time(&self) -> i64 {
        1_790_000_000
    }
    fn delay(&self, _ms: i64, _interruptible: bool) {}
    fn wait_for_input(&self, _ms: i64) -> bool {
        !self.keys.borrow().is_empty()
    }
    fn read_input(&self, buf: &mut [u8]) -> i32 {
        let mut k = self.keys.borrow_mut();
        let n = k.len().min(buf.len());
        buf[..n].copy_from_slice(&k[..n]);
        k.drain(..n);
        n as i32
    }
    fn raise(&self, _sig: i32) {}
    fn suspend(&self) {}
    fn message(&self, msg: &[u8], _err: bool) {
        self.screen.borrow_mut().extend_from_slice(msg);
    }
    fn write(&self, p: &[u8]) -> i32 {
        self.screen.borrow_mut().extend_from_slice(p);
        p.len() as i32
    }
}

fn main() {
    let words = ["alpha", "bravo", "charlie", "delta"];
    let threads: Vec<_> = words
        .iter()
        .map(|w| {
            let w = w.to_string();
            std::thread::spawn(move || {
                let screen = Rc::new(RefCell::new(Vec::new()));
                // its word typed five hundred times, a :%s over the lines, and :q!
                let mut keys = format!("i{w}\x1byy500p:%s/{w}/{w}{w}/\r").into_bytes();
                keys.extend_from_slice(b":q!\r");
                let host = Buffers { keys: RefCell::new(keys), screen: screen.clone() };
                let code = run(Box::new(host), &[b"whimsy".to_vec()]);
                let out = String::from_utf8_lossy(&screen.borrow()).into_owned();
                (w, code, out)
            })
        })
        .collect();
    let mut failed = false;
    for t in threads {
        let (w, code, out) = t.join().unwrap();
        let mine = out.contains(&format!("{w}{w}"));
        let others: Vec<&str> = words.iter().copied().filter(|o| *o != w && out.contains(*o)).collect();
        println!("{w}: status {code}, {} bytes of screen, its own text {}, others' {:?}", out.len(), if mine { "seen" } else { "MISSING" }, others);
        if code != 0 || !mine || !others.is_empty() {
            failed = true;
        }
    }
    if failed {
        std::process::exit(1);
    }
}
