package hv

// Reg is hv_reg_t: Apple's names and values.
type Reg uint32

// HV_REG_X0 .. HV_REG_X30, HV_REG_PC, HV_REG_FPCR, HV_REG_FPSR, HV_REG_CPSR.
const (
	RegX0 Reg = iota
	RegX1
	RegX2
	RegX3
	RegX4
	RegX5
	RegX6
	RegX7
	RegX8
	RegX9
	RegX10
	RegX11
	RegX12
	RegX13
	RegX14
	RegX15
	RegX16
	RegX17
	RegX18
	RegX19
	RegX20
	RegX21
	RegX22
	RegX23
	RegX24
	RegX25
	RegX26
	RegX27
	RegX28
	RegX29
	RegX30
	RegPC
	RegFPCR
	RegFPSR
	RegCPSR

	RegFP = RegX29
	RegLR = RegX30
)

// SysReg is hv_sys_reg_t.  Apple's value for a system register is its
// encoding, op0<<14 | op1<<11 | CRn<<7 | CRm<<3 | op2 -- the same packing as
// KVM's ARM64_SYS_REG, which the KVM backend relies on.
type SysReg uint16

// The HV_SYS_REG_ values used here.
const (
	SysRegSCTLREL1 SysReg = 0xc080
	SysRegCPACREL1 SysReg = 0xc082
	SysRegTTBR0EL1 SysReg = 0xc100
	SysRegTTBR1EL1 SysReg = 0xc101
	SysRegTCREL1   SysReg = 0xc102
	SysRegSPSREL1  SysReg = 0xc200
	SysRegELREL1   SysReg = 0xc201
	SysRegSPEL0    SysReg = 0xc208
	SysRegESREL1   SysReg = 0xc290
	SysRegFAREL1   SysReg = 0xc300
	SysRegMAIREL1  SysReg = 0xc510
	SysRegVBAREL1  SysReg = 0xc600
	SysRegSPEL1    SysReg = 0xe208
)

// RegForSRT is the register a data abort's SRT names: X0..X30 (31, XZR,
// reads as zero and has no Reg; it is reported as RegX0's absence by ok).
func RegForSRT(srt uint32) (Reg, bool) { return Reg(srt), srt < 31 }
