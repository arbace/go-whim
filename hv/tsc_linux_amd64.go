package hv

// kvmGetTSCKHz is KVM_GET_TSC_KHZ, _IO(KVMIO, 0xa3), on a vCPU.
const kvmGetTSCKHz = 0xaea3

// TSCFrequency is the rate of the vCPU's time-stamp counter, in kHz: what a
// guest that keeps its own clock (a Go runtime's nanotime) needs, and has no
// architectural way to ask on amd64 -- arm64's CNTFRQ_EL0 says it there.
// KVM's answer, KVM_GET_TSC_KHZ; not part of Hypervisor.framework's API,
// which has no amd64 guest here.
func TSCFrequency(v VCPU) (uint64, error) {
	c, err := get(v)
	if err != nil {
		return 0, err
	}
	khz, err := ioctl(c.fd, kvmGetTSCKHz, 0)
	if err != nil {
		return 0, fail(Error, "KVM_GET_TSC_KHZ", err)
	}
	return uint64(khz), nil
}
