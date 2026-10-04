package hv

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"
)

// The KVM backend: /dev/kvm through raw ioctl.  The request numbers are the
// kernel's stable ABI (include/uapi/linux/kvm.h), spelled out.
const (
	kvmGetAPIVersion     = 0xae00
	kvmCreateVM          = 0xae01
	kvmCheckExtension    = 0xae03
	kvmGetVCPUMmapSize   = 0xae04
	kvmCreateVCPU        = 0xae41
	kvmSetUserMemRegion  = 0x4020ae46
	kvmRun               = 0xae80
	kvmSetDeviceAttr     = 0x4018aee1
	kvmCapImmediateExit  = 136
	kvmCapSyncRegs       = 74
	kvmMemReadonly       = 1 << 1
	kvmExitUnknown       = 0
	kvmExitIO            = 2
	kvmExitHypercall     = 3
	kvmExitHLT           = 5
	kvmExitMMIO          = 6
	kvmExitShutdown      = 8
	kvmExitFailEntry     = 9
	kvmExitIntr          = 10
	kvmExitInternalError = 17
	kvmExitSystemEvent   = 24
	kvmRunExitReason     = 8   // offsetof(struct kvm_run, exit_reason)
	kvmRunImmediateExit  = 1   // offsetof(struct kvm_run, immediate_exit)
	kvmRunUnion          = 32  // the exit's union
	kvmRunValidRegs      = 288 // kvm_valid_regs
	kvmRunDirtyRegs      = 296 // kvm_dirty_regs
	kvmRunSyncRegs       = 304 // s.regs
	kvmMMIOPhysAddr      = kvmRunUnion
	kvmMMIOData          = kvmRunUnion + 8
	kvmMMIOLen           = kvmRunUnion + 16
	kvmMMIOIsWrite       = kvmRunUnion + 20
	kvmIODirection       = kvmRunUnion
	kvmIOSize            = kvmRunUnion + 1
	kvmIOPort            = kvmRunUnion + 2
	kvmIOCount           = kvmRunUnion + 4
	kvmIODataOffset      = kvmRunUnion + 8
	kvmHypercallNr       = kvmRunUnion
	kvmHypercallArgs     = kvmRunUnion + 8
	kvmHypercallRet      = kvmRunUnion + 56
	kvmHypercallFlags    = kvmRunUnion + 64
	kvmFailEntryReason   = kvmRunUnion
	kvmInternalSuberror  = kvmRunUnion
	kvmSystemEventType   = kvmRunUnion
	ioctlRetries         = 100
)

// the one virtual machine of the process, as the framework has it
var vm struct {
	sync.Mutex
	kvm, fd  int
	created  bool
	slots    map[IPA]memSlot
	nextSlot uint32
	vcpus    []*vcpu
	mmapSize int
	isa      vmISA
}

type memSlot struct {
	slot uint32
	size uint64
}

// vcpu is a vCPU's KVM state: its fd, its kvm_run page, the exit record
// handed out, the thread it lives on, and the ISA's register caches.
type vcpu struct {
	fd       int
	run      []byte
	exit     *VCPUExit
	tid      int
	cancel   atomic.Bool
	isa      vcpuISA
	runs     uint64
	spurious uint64 // KVM_RUN interrupted by a signal and run again
}

func ioctl(fd int, req, arg uintptr) (uintptr, error) {
	for range ioctlRetries {
		r, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), req, arg)
		if e == syscall.EINTR || e == syscall.EAGAIN {
			continue
		}
		if e != 0 {
			return r, e
		}
		return r, nil
	}
	return 0, syscall.EINTR
}

func fail(r Return, op string, err error) error { return &errnoError{r, op, err} }

// VMCreate is hv_vm_create: the process's one virtual machine.
func VMCreate(config *VMConfig) error {
	vm.Lock()
	defer vm.Unlock()
	if vm.created {
		return Busy
	}
	kvm, err := syscall.Open("/dev/kvm", syscall.O_RDWR|syscall.O_CLOEXEC, 0)
	if err != nil {
		return fail(NoDevice, "open /dev/kvm", err)
	}
	if v, err := ioctl(kvm, kvmGetAPIVersion, 0); err != nil || v != 12 {
		syscall.Close(kvm)
		return fail(Unsupported, "KVM_GET_API_VERSION", fmt.Errorf("%d, %v", v, err))
	}
	if ok, _ := ioctl(kvm, kvmCheckExtension, kvmCapImmediateExit); ok == 0 {
		syscall.Close(kvm)
		return fail(Unsupported, "KVM_CAP_IMMEDIATE_EXIT", syscall.ENOSYS)
	}
	typ := uintptr(0)
	if config != nil {
		typ = vmType(config.IPASize)
	}
	fd, err := ioctl(kvm, kvmCreateVM, typ)
	if err != nil {
		syscall.Close(kvm)
		return fail(NoResources, "KVM_CREATE_VM", err)
	}
	sz, err := ioctl(kvm, kvmGetVCPUMmapSize, 0)
	if err != nil {
		syscall.Close(int(fd))
		syscall.Close(kvm)
		return fail(Error, "KVM_GET_VCPU_MMAP_SIZE", err)
	}
	syscall.CloseOnExec(int(fd))
	vm.kvm, vm.fd, vm.mmapSize = kvm, int(fd), int(sz)
	vm.slots = map[IPA]memSlot{}
	vm.nextSlot = 0
	vm.vcpus = nil
	vm.isa = vmISA{}
	if err := vmInit(); err != nil {
		syscall.Close(vm.fd)
		syscall.Close(vm.kvm)
		return err
	}
	vm.created = true
	return nil
}

// VMDestroy is hv_vm_destroy.  Its vCPUs must have been destroyed.
func VMDestroy() error {
	vm.Lock()
	defer vm.Unlock()
	if !vm.created {
		return BadArgument
	}
	for _, c := range vm.vcpus {
		if c != nil {
			return Busy
		}
	}
	syscall.Close(vm.fd)
	syscall.Close(vm.kvm)
	vm.created = false
	return nil
}

// kvmUserspaceMemoryRegion is struct kvm_userspace_memory_region.
type kvmUserspaceMemoryRegion struct {
	slot, flags              uint32
	guestPhys, size, userptr uint64
}

// VMMap is hv_vm_map: mem, page-aligned, as guest RAM at ipa.  KVM has no
// execute permission to withhold: MemoryExec is the guest's own page
// tables' business there.  A map without MemoryWrite is read-only.
func VMMap(mem []byte, ipa IPA, flags MemoryFlags) error {
	vm.Lock()
	defer vm.Unlock()
	if !vm.created {
		return BadArgument
	}
	if len(mem) == 0 || uintptr(unsafe.Pointer(&mem[0]))%uintptr(os.Getpagesize()) != 0 || uint64(ipa)%uint64(os.Getpagesize()) != 0 {
		return BadArgument
	}
	r := kvmUserspaceMemoryRegion{slot: vm.nextSlot, guestPhys: uint64(ipa), size: uint64(len(mem)),
		userptr: uint64(uintptr(unsafe.Pointer(&mem[0])))}
	if flags&MemoryWrite == 0 {
		r.flags |= kvmMemReadonly
	}
	if _, err := ioctl(vm.fd, kvmSetUserMemRegion, uintptr(unsafe.Pointer(&r))); err != nil {
		return fail(Error, "KVM_SET_USER_MEMORY_REGION", err)
	}
	vm.slots[ipa] = memSlot{vm.nextSlot, uint64(len(mem))}
	vm.nextSlot++
	return nil
}

// VMUnmap is hv_vm_unmap: the mapping made at ipa, of size bytes, removed.
func VMUnmap(ipa IPA, size uint64) error {
	vm.Lock()
	defer vm.Unlock()
	s, ok := vm.slots[ipa]
	if !ok || s.size != size {
		return BadArgument
	}
	r := kvmUserspaceMemoryRegion{slot: s.slot, guestPhys: uint64(ipa)}
	if _, err := ioctl(vm.fd, kvmSetUserMemRegion, uintptr(unsafe.Pointer(&r))); err != nil {
		return fail(Error, "KVM_SET_USER_MEMORY_REGION", err)
	}
	delete(vm.slots, ipa)
	return nil
}

// VCPUCreate is hv_vcpu_create: a vCPU on the calling thread, which must be
// locked to it (runtime.LockOSThread) and is the only one to run it, and
// the exit record VCPURun fills.
func VCPUCreate() (VCPU, *VCPUExit, error) {
	vm.Lock()
	defer vm.Unlock()
	if !vm.created {
		return 0, nil, BadArgument
	}
	id := len(vm.vcpus)
	fd, err := ioctl(vm.fd, kvmCreateVCPU, uintptr(id))
	if err != nil {
		return 0, nil, fail(NoResources, "KVM_CREATE_VCPU", err)
	}
	syscall.CloseOnExec(int(fd))
	run, err := syscall.Mmap(int(fd), 0, vm.mmapSize, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		syscall.Close(int(fd))
		return 0, nil, fail(NoResources, "mmap kvm_run", err)
	}
	c := &vcpu{fd: int(fd), run: run, exit: &VCPUExit{}, tid: syscall.Gettid()}
	if err := vcpuInit(c); err != nil {
		syscall.Munmap(run)
		syscall.Close(int(fd))
		return 0, nil, err
	}
	vm.vcpus = append(vm.vcpus, c)
	return VCPU(id), c.exit, nil
}

func get(v VCPU) (*vcpu, error) {
	vm.Lock()
	defer vm.Unlock()
	if int(v) >= len(vm.vcpus) || vm.vcpus[v] == nil {
		return nil, BadArgument
	}
	return vm.vcpus[v], nil
}

// VCPUDestroy is hv_vcpu_destroy.  It holds the VM's lock, so that a
// VCPUsExit from another thread finds the vCPU whole or not at all.
func VCPUDestroy(v VCPU) error {
	vm.Lock()
	defer vm.Unlock()
	if int(v) >= len(vm.vcpus) || vm.vcpus[v] == nil {
		return BadArgument
	}
	c := vm.vcpus[v]
	vm.vcpus[v] = nil
	syscall.Munmap(c.run)
	syscall.Close(c.fd)
	return nil
}

// VCPUsExit is hv_vcpus_exit: each vCPU named, running or about to, returns
// from VCPURun with ExitReasonCanceled.  KVM's way: immediate_exit set in
// its kvm_run, and a signal to its thread to end a KVM_RUN under way --
// SIGURG, which the Go runtime takes for a preemption request it did not
// make, and ignores.
func VCPUsExit(vcpus ...VCPU) error {
	vm.Lock()
	defer vm.Unlock()
	for _, v := range vcpus {
		if int(v) >= len(vm.vcpus) || vm.vcpus[v] == nil {
			return BadArgument
		}
		c := vm.vcpus[v]
		c.cancel.Store(true)
		atomic.StoreUint32((*uint32)(unsafe.Pointer(&c.run[0])), 1<<8) // immediate_exit = 1
		syscall.Tgkill(syscall.Getpid(), c.tid, syscall.SIGURG)
	}
	return nil
}

// VCPURun is hv_vcpu_run: the vCPU runs until it exits, and its exit record
// says why.  A KVM_RUN interrupted by a signal the caller did not ask for
// (VCPUsExit) is run again, as the framework would not have returned.
func VCPURun(v VCPU) error {
	c, err := get(v)
	if err != nil {
		return err
	}
	if c.tid != syscall.Gettid() {
		return fail(BadArgument, "hv_vcpu_run", fmt.Errorf("vCPU %d created on thread %d, run on %d (runtime.LockOSThread)", v, c.tid, syscall.Gettid()))
	}
	if err := beforeRun(c); err != nil {
		return err
	}
	for {
		c.runs++
		_, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(c.fd), kvmRun, 0)
		if e == syscall.EINTR || e == syscall.EAGAIN || (e == 0 && c.u32(kvmRunExitReason) == kvmExitIntr) {
			if c.cancel.Swap(false) {
				atomic.StoreUint32((*uint32)(unsafe.Pointer(&c.run[0])), 0)
				*c.exit = VCPUExit{Reason: ExitReasonCanceled}
				afterRun(c, false)
				return nil
			}
			c.spurious++
			continue
		}
		if e != 0 {
			return fail(Error, "KVM_RUN", e)
		}
		break
	}
	*c.exit = VCPUExit{}
	afterRun(c, true)
	reason := c.u32(kvmRunExitReason)
	switch reason {
	case kvmExitMMIO:
		n := int(c.u32(kvmMMIOLen))
		write := c.run[kvmMMIOIsWrite] != 0
		var data uint64
		for i := n - 1; i >= 0; i-- {
			data = data<<8 | uint64(c.run[kvmMMIOData+i])
		}
		c.exit.Reason = ExitReasonException
		c.exit.Exception.PhysicalAddress = IPA(c.u64(kvmMMIOPhysAddr))
		c.exit.Exception.Syndrome = uint64(mmioExit(c, n, write, data))
		return nil
	}
	if isaExit(c, reason) {
		return nil
	}
	c.exit.Reason = ExitReasonUnknown
	c.exit.Detail = describe(c, reason)
	return nil
}

func describe(c *vcpu, reason uint32) string {
	switch reason {
	case kvmExitHLT:
		return "KVM_EXIT_HLT"
	case kvmExitShutdown:
		return "KVM_EXIT_SHUTDOWN (a triple fault)"
	case kvmExitFailEntry:
		return fmt.Sprintf("KVM_EXIT_FAIL_ENTRY, hardware reason %#x", c.u64(kvmFailEntryReason))
	case kvmExitInternalError:
		return fmt.Sprintf("KVM_EXIT_INTERNAL_ERROR, suberror %d", c.u32(kvmInternalSuberror))
	case kvmExitSystemEvent:
		return fmt.Sprintf("KVM_EXIT_SYSTEM_EVENT, type %d", c.u32(kvmSystemEventType))
	case kvmExitIO:
		return fmt.Sprintf("KVM_EXIT_IO, port %#x", c.u16(kvmIOPort))
	case kvmExitHypercall:
		return fmt.Sprintf("KVM_EXIT_HYPERCALL, nr %#x", c.u64(kvmHypercallNr))
	case kvmExitUnknown:
		return "KVM_EXIT_UNKNOWN"
	}
	return fmt.Sprintf("KVM exit reason %d", reason)
}

func (c *vcpu) u16(off int) uint16 { return *(*uint16)(unsafe.Pointer(&c.run[off])) }
func (c *vcpu) u32(off int) uint32 { return *(*uint32)(unsafe.Pointer(&c.run[off])) }
func (c *vcpu) u64(off int) uint64 { return *(*uint64)(unsafe.Pointer(&c.run[off])) }

// Stats is what the backend counted on v: KVM_RUN calls, and those a signal
// interrupted that were run again.  Not the framework's.
func Stats(v VCPU) (runs, spurious uint64) {
	c, err := get(v)
	if err != nil {
		return 0, 0
	}
	return c.runs, c.spurious
}

// LockThread is the framework's rule made explicit for Go: the goroutine
// that creates a vCPU is the one that runs it, on one thread for the vCPU's
// life.
func LockThread() { runtime.LockOSThread() }

// errnoError is a backend's failure with its cause kept: a Return for the
// framework's caller, the system's error for a person.
type errnoError struct {
	r    Return
	op   string
	errn error
}

func (e *errnoError) Error() string { return fmt.Sprintf("%s: %s: %v", e.r, e.op, e.errn) }
func (e *errnoError) Unwrap() error { return e.r }
