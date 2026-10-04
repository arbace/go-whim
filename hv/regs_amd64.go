package hv

// Reg is a general register, RIP or RFLAGS: x86 names behind the
// framework's shape (Apple's x86 API is not the model).  RAX is 0, so that
// a doorbell store's SRT of 0 names the register the store read (the
// guest's ABI stores from RAX).
type Reg uint32

// The x86-64 registers, in KVM's struct kvm_regs order.
const (
	RegRAX Reg = iota
	RegRBX
	RegRCX
	RegRDX
	RegRSI
	RegRDI
	RegRSP
	RegRBP
	RegR8
	RegR9
	RegR10
	RegR11
	RegR12
	RegR13
	RegR14
	RegR15
	RegRIP
	RegRFLAGS
)

// SysReg is a control register, EFER, a descriptor table's base or limit, or
// one of a segment's four fields: what KVM keeps in struct kvm_sregs.
type SysReg uint16

// The system registers.
const (
	SysRegCR0 SysReg = iota + 1
	SysRegCR2
	SysRegCR3
	SysRegCR4
	SysRegEFER
	SysRegGDTRBase
	SysRegGDTRLimit
	SysRegIDTRBase
	SysRegIDTRLimit
)

// Segment is a segment register, in struct kvm_sregs' order.
type Segment uint16

// The segments.
const (
	SegCS Segment = iota
	SegDS
	SegES
	SegFS
	SegGS
	SegSS
	SegTR
	SegLDTR
)

// The four fields of a segment, as VMX names them.
const (
	segSelector = iota
	segBase
	segLimit
	segAR
)

const segFirst SysReg = 0x100

// Selector, Base, Limit and AR are the segment's fields as system registers.
// AR is VMX's access-rights encoding: type in bits 0-3, S 4, DPL 5-6, P 7,
// AVL 12, L 13, D/B 14, G 15, unusable 16.
func (s Segment) Selector() SysReg { return segFirst + SysReg(s)*4 + segSelector }
func (s Segment) Base() SysReg     { return segFirst + SysReg(s)*4 + segBase }
func (s Segment) Limit() SysReg    { return segFirst + SysReg(s)*4 + segLimit }
func (s Segment) AR() SysReg       { return segFirst + SysReg(s)*4 + segAR }

// RegForSRT is the register a data abort's SRT names; the KVM backend
// reports 0, RAX, for a store whose value RAX holds.
func RegForSRT(srt uint32) (Reg, bool) { return Reg(srt), srt <= uint32(RegR15) }
